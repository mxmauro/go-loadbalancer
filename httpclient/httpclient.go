package httpclient

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	balancer "github.com/randlabs/go-loadbalancer"
)

// -----------------------------------------------------------------------------

const (
	ServerUpEvent int = iota + 1
	ServerDownEvent
)

var ErrCanceled = errors.New("canceled")
var ErrTimeout = errors.New("timeout")

// -----------------------------------------------------------------------------

// HttpClient is a load-balancer http client requester object.
type HttpClient struct {
	lb           *balancer.LoadBalancer
	transport    *http.Transport
	eventHandler EventHandler
	sources      []*Source
}

// SourceState indicates the state of a server.
type SourceState struct {
	BaseURL   string
	IsBackup  bool
	LastError error
}

// SourceOptions specifies details about a source.
type SourceOptions balancer.ServerOptions

// EventHandler is a handler for load balancer events.
type EventHandler func(eventType int, source *Source, err error)

// -----------------------------------------------------------------------------

// Create creates a load-balanced http client requester object.
func Create() *HttpClient {
	// From: https://www.loginradius.com/blog/async/tune-the-go-http-client-for-high-performance/
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxConnsPerHost = 100
	transport.IdleConnTimeout = 60 * time.Second
	transport.MaxIdleConnsPerHost = 100
	transport.ResponseHeaderTimeout = 5 * time.Second
	return CreateWithTransport(transport)
}

// CreateWithTransport creates a load-balanced http client requester object that uses the specified transport.
func CreateWithTransport(transport *http.Transport) *HttpClient {
	c := HttpClient{
		lb:        balancer.Create(),
		transport: transport.Clone(),
		sources:   make([]*Source, 0),
	}
	c.lb.SetEventHandler(c.balancerEventHandler)

	// Done
	return &c
}

// AddSource adds a new source to the load-balanced http client object.
func (c *HttpClient) AddSource(baseURL string, headers map[string]string, opts SourceOptions) error {
	// Check base url
	match, _ := regexp.MatchString(`https?://([^:/?#]+)(:\d+)?/?$`, baseURL)
	if !match {
		return errors.New("missing base url")
	}

	// Remove trailing slash
	if strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL[0 : len(baseURL)-1]
	}

	// Add source to list
	src := &Source{
		id:           len(c.sources) + 1,
		baseURL:      baseURL,
		headers:      make(map[string]string),
		isBackup:     opts.IsBackup,
		lastErrorMtx: sync.RWMutex{},
		lastError:    nil,
	}
	for k,v := range headers {
		src.headers[k] = v
	}

	c.sources = append(c.sources, src)

	// Add source to the load balancer
	err := c.lb.Add(balancer.ServerOptions(opts), src)
	if err != nil {
		// On error, remove the source from the source list
		c.sources = c.sources[0:len(c.sources)-1]
		return err
	}

	// Done
	return nil
}

// SourcesCount retrieves the number of sources
func (c *HttpClient) SourcesCount() int {
	return len(c.sources)
}

// SourceState retrieves source details
func (c *HttpClient) SourceState(index int) *SourceState {
	if index < 0 || index >= len(c.sources) {
		return nil
	}
	ss := SourceState{
		BaseURL:   c.sources[index].BaseURL(),
		IsBackup:  c.sources[index].IsBackup(),
		LastError: c.sources[index].Err(),
	}
	return &ss
}

// SetEventHandler sets a new notification handler callback
func (c *HttpClient) SetEventHandler(handler EventHandler) {
	c.eventHandler = handler
}
