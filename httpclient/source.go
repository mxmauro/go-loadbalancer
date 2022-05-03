package httpclient

import (
	"sync"
)

// -----------------------------------------------------------------------------

type Source struct {
	id           int // NOTE: The IDs starts from 1
	baseURL      string
	headers      map[string]string
	isBackup     bool
	lastErrorMtx sync.RWMutex
	lastError    error
}

// -----------------------------------------------------------------------------

func (src *Source) ID() int {
	return src.id
}

func (src *Source) BaseURL() string {
	return src.baseURL
}

func (src *Source) IsBackup() bool {
	return src.isBackup
}

func (src *Source) Err() error {
	var err error

	src.lastErrorMtx.RLock()
	err = src.lastError
	src.lastErrorMtx.RUnlock()
	return err
}

func (src *Source) setLastError(err error) {
	src.lastErrorMtx.RLock()
	src.lastError = err
	src.lastErrorMtx.RUnlock()
}
