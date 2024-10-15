// See the LICENSE file for license details.

package httpclient

import (
	"errors"

	"github.com/mxmauro/go-loadbalancer/v2"
)

// -----------------------------------------------------------------------------

var errServerDown = errors.New("server down")

// -----------------------------------------------------------------------------

func (c *HttpClient) balancerEventHandler(eventType int, srv *loadbalancer.Server) {
	src := srv.UserData().(*Source)

	// Set the source online status based on the received event and notify the upper event handler
	switch eventType {
	case loadbalancer.ServerUpEvent:
		src.setOnlineStatus(true)
		if c.eventHandler != nil {
			c.eventHandler(ServerUpEvent, src.ID(), nil)
		}

	case loadbalancer.ServerDownEvent:
		src.setOnlineStatus(false)
		if c.eventHandler != nil {
			c.eventHandler(ServerDownEvent, src.ID(), errServerDown)
		}
	}
}

func (c *HttpClient) raiseRequestEvent(srv *loadbalancer.Server, err error) {
	if c.eventHandler != nil {
		src := srv.UserData().(*Source)
		if err == nil {
			c.eventHandler(RequestSucceededEvent, src.ID(), nil)
		} else {
			c.eventHandler(RequestFailedEvent, src.ID(), err)
		}
	}
}
