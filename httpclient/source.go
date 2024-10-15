// See the LICENSE file for license details.

package httpclient

import (
	"net/http"
	"sync/atomic"
)

// -----------------------------------------------------------------------------

// Source represents a server where the client will do requests.
type Source struct {
	id        int // NOTE: The IDs starts from 1
	baseURL   string
	header    http.Header
	isBackup  bool
	isOnline  int32
	lastError atomic.Value
}

// Hack-hack to avoid panics on atomic.Value
type packedError struct {
	err error
}

// -----------------------------------------------------------------------------

func newSource(id int, baseURL string, headers http.Header, isBackup bool) *Source {
	src := Source{
		id:        id,
		baseURL:   baseURL,
		header:    headers.Clone(),
		isBackup:  isBackup,
		lastError: atomic.Value{},
	}
	atomic.StoreInt32(&src.isOnline, 1)
	src.setLastError(nil)

	return &src
}

// ID returns the source identifier.
func (src *Source) ID() int {
	return src.id
}

// BaseURL returns the source base url.
func (src *Source) BaseURL() string {
	return src.baseURL
}

// IsBackup returns if the source is primary or backup.
func (src *Source) IsBackup() bool {
	return src.isBackup
}

// IsOnline returns if the source is online.
func (src *Source) IsOnline() bool {
	return atomic.LoadInt32(&src.isOnline) != 0
}

// Err returns the last error occurred in the source.
func (src *Source) Err() error {
	perr := src.lastError.Load().(packedError)
	return perr.err
}

func (src *Source) setOnlineStatus(online bool) {
	if online {
		atomic.StoreInt32(&src.isOnline, 1)
	} else {
		atomic.StoreInt32(&src.isOnline, 0)
	}
}

func (src *Source) setLastError(err error) {
	src.lastError.Store(packedError{
		err: err,
	})
}
