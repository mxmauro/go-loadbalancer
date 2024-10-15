// See the LICENSE file for license details.

package httpclient

import (
	"errors"
	"fmt"
	"net"
)

// -----------------------------------------------------------------------------

const (
	errorTypeIsTimeout = 1
	errorTypeIsCanceled = 2
)

// -----------------------------------------------------------------------------

// Error is the error type usually returned by us.
type Error struct {
	message    string
	url        string
	statusCode int
	errType    int
	// err is the underlying error that occurred during the operation.
	err        error
}

// -----------------------------------------------------------------------------

func (c *HttpClient) newError(wrappedErr error, message string, url string, statusCode int) *Error {
	err := Error{
		message:    message,
		url:        url,
		statusCode: statusCode,
		err:        wrappedErr,
	}
	return &err
}

// -----------------------------------------------------------------------------

func (e *Error) URL() string {
	return e.url
}

func (e *Error) StatusCode() int {
	return e.statusCode
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	s := e.message + fmt.Sprintf(" [URL=%v]", e.url)
	if e.err != nil {
		s += " [err=" + e.err.Error() + "]"
	}
	return s
}

func (e *Error) IsTimeout() bool {
	return e.errType == errorTypeIsTimeout
}

func (e *Error) IsCanceled() bool {
	return e.errType == errorTypeIsCanceled
}

func (e *Error) IsNetworkError() bool {
	if e.err != nil {
		var netErr net.Error
		var netOpErr *net.OpError
		var netDnsErr *net.DNSError

		if errors.As(e.err, &netErr) || errors.As(e.err, &netOpErr)  || errors.As(e.err, &netDnsErr) {
			return true
		}
	}
	return false
}
