package httpclient

import (
	"context"
	"net/http"
)

// -----------------------------------------------------------------------------

// ExecCallback specifies a callback to call when a request completes with success or failure.
type ExecCallback func(ctx context.Context, res Response) error

// Response contains details about the executed request.
type Response struct {
	// Response has the response details. Nil if the request failed to complete (see also Err method)
	// IMPORTANT NOTE: The response body, if present, WILL BE CLOSED by the Exec method body
	*http.Response

	fullUrl         string
	source          *Source
	retryCount      int
	err             error
	upstreamOffline *bool
	retry           *bool
}

// -----------------------------------------------------------------------------

// URL returns the base url for the selected server plus the resource uri.
func (res *Response) URL() string {
	return res.fullUrl
}

// Err returns the error of a failed request. Non 2xx status code are not considered an error
func (res *Response) Err() error {
	return res.err
}

// RetryCount has the number of retries of the current request.
func (res *Response) RetryCount() int {
	return res.retryCount
}

// SetOffline indicates the accessed server must be considered to be offline.
func (res *Response) SetOffline() {
	*res.upstreamOffline = true
}

// RetryOnNextServer indicates the request must be retried on the next available server.
func (res *Response) RetryOnNextServer() {
	*res.retry = true
}

// SourceID indicates the request must be retried on the next available server.
func (res *Response) SourceID() int {
	return res.source.ID()
}

// SourceBaseURL returns the base URL to use.
func (res *Response) SourceBaseURL() string {
	return res.source.baseURL
}
