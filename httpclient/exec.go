package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

// -----------------------------------------------------------------------------

const (
	errUnableToExecuteRequest = "failed to execute http request"
	errNoAvailableServer      = "no available upstream server"
)

// -----------------------------------------------------------------------------

func (c *HttpClient) exec(req *Request) error {
	var httpReq *http.Request
	var getBody func() io.ReadCloser
	var err error

	// Define a body getter to return multiple copies of the reader to be used in retries.
	if req.body == nil {
		// If no body, getter will return nil
		getBody = func() io.ReadCloser {
			return nil
		}
	} else {
		// Convert to a ReadCloser if just a reader
		rc, ok := req.body.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(req.body)
		}

		// Defer close of the original body
		defer func() {
			_ = rc.Close()
		}()

		// Set up a body reader cloning function
		switch v := req.body.(type) {
		case *bytes.Buffer:
			buf := v.Bytes()
			getBody = func() io.ReadCloser {
				r := bytes.NewReader(buf)
				return io.NopCloser(r)
			}

		case *bytes.Reader:
			snapshot := *v
			getBody = func() io.ReadCloser {
				r := snapshot
				return io.NopCloser(&r)
			}

		case *strings.Reader:
			snapshot := *v
			getBody = func() io.ReadCloser {
				r := snapshot
				return io.NopCloser(&r)
			}

		default:
			return errors.New("unsupported body reader")
		}
	}

	// Initialize retry counter
	retryCounter := 0

	// Loop
	for {
		var netErr net.Error

		// Get next available server
		srv := c.lb.Next()
		if srv == nil {
			return c.newError(nil, errNoAvailableServer, req.url, 0)
		}

		src := srv.UserData().(*Source)

		// Create the final url
		url := src.baseURL + req.url

		// Create a new http request
		httpReq, err = http.NewRequest(req.method, url, getBody())
		if err != nil {
			err = c.newError(err, errUnableToExecuteRequest, url, 0)
			src.setLastError(err)
			return err
		}

		// Add load balancer source headers
		httpReq.Header = src.header.Clone()

		// Add request headers
		if req.headers != nil {
			for k, v := range req.headers {
				vLen := len(v)
				if vLen > 0 {
					httpReq.Header.Set(k, v[0])
					for vIdx := 1; vIdx < vLen; vIdx++ {
						httpReq.Header.Add(k, v[vIdx])
					}
				}
			}
		}

		// Create http client requester
		client := http.Client{
			Transport: c.transport,
		}

		// Build callback info
		upstreamOffline := false
		retry := false
		execResult := Response{
			fullUrl:         url,
			source:          src,
			retryCount:      retryCounter,
			upstreamOffline: &upstreamOffline,
			retry:           &retry,
		}

		// Establish a new context with the timeout
		ctx, cancelCtx := context.WithTimeout(req.ctx, req.timeout)

		// Execute real request
		execResult.Response, err = client.Do(httpReq.WithContext(ctx))
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// Deadline exceeded?
				err = ErrTimeout
			} else if errors.As(err, &netErr) && netErr.Timeout() {
				// Network timeout?
				srv.SetOffline()

				err = ErrTimeout
			} else if errors.Is(err, context.Canceled) {
				// Canceled?
				err = ErrCanceled
			} else {
				// Other type of error
				srv.SetOffline()

				err = c.newError(err, errUnableToExecuteRequest, url, 0)
			}
		}

		// Set error in callback
		execResult.err = err

		// Call the callback
		err = req.callback(ctx, execResult)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				err = ErrTimeout
			} else if errors.As(err, &netErr) && netErr.Timeout() {
				err = ErrTimeout
			} else if errors.Is(err, context.Canceled) {
				err = ErrCanceled
			}
		}

		// To avoid defer calling inside a for loop and warnings, we call it here
		cancelCtx()

		// Close the response body if one exist
		if execResult.Response != nil {
			_ = execResult.Response.Body.Close()
		}

		// Set the last error (even success)
		src.setLastError(err)

		// Raise callback
		c.raiseRequestEvent(srv, err)

		// Set server online/offline based on the callback response
		if !upstreamOffline {
			srv.SetOnline()
		} else {
			srv.SetOffline()
		}

		// Should we retry on next server?
		if !retry {
			break
		}

		// Increment retry counter
		retryCounter += 1
	}

	// Done
	return err
}
