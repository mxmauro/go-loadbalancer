package httpclient

import (
	"errors"

	balancer "github.com/randlabs/go-loadbalancer"
)

// -----------------------------------------------------------------------------

var errServerDown = errors.New("server down")

// -----------------------------------------------------------------------------

func (c *HttpClient) balancerEventHandler(eventType int, srv *balancer.Server) {
	if c.eventHandler != nil {
		src := srv.UserData().(*Source)

		switch eventType {
		case balancer.ServerUpEvent:
			c.eventHandler(ServerUpEvent, src, nil)

		case balancer.ServerDownEvent:
			c.eventHandler(ServerDownEvent, src, errServerDown)
		}
	}
}
