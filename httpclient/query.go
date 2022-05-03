package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// -----------------------------------------------------------------------------

type QueryCallbackInfo struct {
	FullUrl         string
	Response        *http.Response
	RetryCount      int
	Err             error

	upstreamOffline bool
	retry           bool
}

type QueryCallback func(info QueryCallbackInfo) error

// -----------------------------------------------------------------------------

const (
	defaultTimeout = 20 * time.Second

	errUnableToExecuteRequest = "failed to execute http request"
	errNoAvailableServer      = "no available upstream server"
)

// -----------------------------------------------------------------------------

func (c *HttpClient) exec(ctx context.Context, cb QueryCallback, req *Request) error {
	var httpReq *http.Request
	var cancelCtx context.CancelFunc
	var err error

	retryCounter := 0

	for {
		// Get next available server
		srv := c.lb.Next()
		if srv == nil {
			return c.buildError(nil, nil, errNoAvailableServer, req.Resource, 0)
		}

		src := srv.UserData().(*Source)

		// Create the final url
		url := src.baseURL + req.Resource

		// Create a new http request
		httpReq, err = http.NewRequest(req.Method, url, req.Body)
		if err != nil {
			return c.buildError(src, err, errUnableToExecuteRequest, url, 0)
		}

		// Add load balancer source headers
		for k, v := range src.headers {
			httpReq.Header.Set(k, v)
		}

		// Add request headers
		for k, v := range req.Header {
			vLen := len(v)
			if vLen > 0 {
				httpReq.Header.Set(k, v[0])
				for vIdx := 1; vIdx < vLen; vIdx++ {
					httpReq.Header.Add(k, v[vIdx])
				}
			}
		}

		// Create http client requester
		client := http.Client{
			Transport: c.transport,
		}

		// Establish a new context with a default timeout if a deadline is not present
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			ctx, cancelCtx = context.WithTimeout(ctx, defaultTimeout)
		} else {
			cancelCtx = nil
		}

		// Build callback info
		callbackInfo := QueryCallbackInfo{
			FullUrl:    url,
			RetryCount: retryCounter,
		}

		// Execute real request
		callbackInfo.Response, err = client.Do(httpReq.WithContext(ctx))
		if err != nil {
			var netErr net.Error

			if errors.Is(err, context.DeadlineExceeded) {
				// Deadline exceeded?
				srv.SetOffline()

				callbackInfo.Err = ErrTimeout
			} else if errors.As(err, &netErr); netErr.Timeout() {
				// Network timeout?
				srv.SetOffline()

				callbackInfo.Err = ErrTimeout
			} else if errors.Is(err, context.Canceled) {
				// Canceled?
				return ErrCanceled
			} else {
				// Other type of error
				srv.SetOffline()

				callbackInfo.Err = c.buildError(src, err, errUnableToExecuteRequest, url, 0)
			}
		}

		// To avoid defer calling inside a for loop and warnings, we call it here
		if cancelCtx != nil {
			cancelCtx()
		}

		// Set error
		callbackInfo.Err = err

		// Call the callback
		err = cb(callbackInfo)

		// Set the last error (even success)
		src.setLastError(err)

		// Set server online/offline based on the callback response
		if !callbackInfo.upstreamOffline {
			srv.SetOnline()
		} else {
			srv.SetOffline()
		}

		// Should we retry on next server?
		if !callbackInfo.retry {
			break
		}

		// Increment retry counter
		retryCounter += 1
	}

	// Done
	return err
}

// -----------------------------------------------------------------------------

func (info *QueryCallbackInfo) SetOffline() {
	info.upstreamOffline = true
}

func (info *QueryCallbackInfo) RetryOnNextServer() {
	info.retry = true
}
