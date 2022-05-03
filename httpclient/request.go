package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// -----------------------------------------------------------------------------

type Request struct {
	client   *HttpClient
	Method   string
	Resource string
	Header   http.Header
	Body     io.Reader
}

// -----------------------------------------------------------------------------

func (c *HttpClient) NewRequest(method string, resourceUrl string) *Request {
	req := Request{
		client:   c,
		Method:   method,
		Resource: resourceUrl,
		Header:   make(http.Header),
	}
	return &req
}

func (req *Request) SetBody(body io.Reader) {
	req.Body = body
}

func (req *Request) SetBodyBytes(body []byte) {
	if body != nil {
		req.Body = bytes.NewReader(body)
	} else {
		req.Body = nil
	}
}

func (req *Request) Exec(ctx context.Context, cb QueryCallback) error {
	return req.client.exec(ctx, cb, req)
}

