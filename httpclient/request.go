package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// -----------------------------------------------------------------------------

// Request represents a load-balanced http client request object.
type Request struct {
	Method   string
	Resource string
	Header   http.Header
	Body     io.Reader

	client   *HttpClient
}

// -----------------------------------------------------------------------------

// NewRequest creates a new http client request
func (c *HttpClient) NewRequest(method string, resourceUrl string) *Request {
	req := Request{
		client:   c,
		Method:   method,
		Resource: resourceUrl,
		Header:   make(http.Header),
	}
	return &req
}

// SetBody sets the body of a http client request
func (req *Request) SetBody(body io.Reader) {
	req.Body = body
}

// SetBodyBytes sets the body of a http client request
func (req *Request) SetBodyBytes(body []byte) {
	if body != nil {
		req.SetBody(bytes.NewReader(body))
	} else {
		req.Body = nil
	}
}

// Exec runs the http client request
func (req *Request) Exec(ctx context.Context, cb ExecCallback) error {
	return req.client.exec(ctx, cb, req)
}
