package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// -----------------------------------------------------------------------------

const (
	defaultTimeout = 20 * time.Second
)

// -----------------------------------------------------------------------------

// Request represents a load-balanced http client request object.
type Request struct {
	method  string
	url     string
	headers http.Header
	body    io.Reader
	ctx context.Context
	timeout time.Duration
	callback ExecCallback
	client  *HttpClient
}

// -----------------------------------------------------------------------------

// NewRequest creates a new http client request
func (c *HttpClient) NewRequest(ctx context.Context, url string) *Request {
	if ctx == nil {
		ctx = context.Background()
	}
	req := Request{
		ctx:     ctx,
		client:  c,
		timeout: defaultTimeout,
		method:  "GET",
		url:     url,
	}
	return &req
}

// Method sets the http client request method to use
func (req *Request) Method(method string) *Request {
	req.method = method
	return req
}

// Headers sets the headers of a http client request
func (req *Request) Headers(headers http.Header) *Request {
	req.headers = headers
	return req
}

// Body sets the body of a http client request
func (req *Request) Body(body io.Reader) *Request {
	req.body = body
	return req
}

// BodyBytes sets the body of a http client request
func (req *Request) BodyBytes(body []byte) *Request {
	if body != nil {
		req.body = bytes.NewReader(body)
	} else {
		req.body = nil
	}
	return req
}

// Timeout sets the request timeout
func (req *Request) Timeout(timeout time.Duration) *Request {
	req.timeout = timeout
	return req
}

// Callback sets the execution callback
func (req *Request) Callback(cb ExecCallback) *Request {
	req.callback = cb
	return req
}

// Exec runs the http client request
func (req *Request) Exec() error {
	if len(req.method) == 0 {
		return errors.New("invalid method")
	}
	if len(req.url) == 0 {
		return errors.New("invalid url")
	}
	if req.timeout < 0 {
		return errors.New("invalid timeout")
	}
	if req.callback == nil {
		return errors.New("invalid callback")
	}
	return req.client.exec(req)
}
