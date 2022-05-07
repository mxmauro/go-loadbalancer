package httpclient

import (
	"errors"

	balancer "github.com/randlabs/go-loadbalancer"
)

// -----------------------------------------------------------------------------

var serverDownErr = errors.New("server down")

// -----------------------------------------------------------------------------

func (c *HttpClient) balancerEventHandler(eventType int, srv *balancer.Server) {
	src := srv.UserData().(*Source)

	// Set the source online status based on the received event and notify the upper event handler
	switch eventType {
	case balancer.ServerUpEvent:
		src.setOnlineStatus(true)
		if c.eventHandler != nil {
			c.eventHandler(ServerUpEvent, src.ID(), nil)
		}

	case balancer.ServerDownEvent:
		src.setOnlineStatus(false)
		if c.eventHandler != nil {
			c.eventHandler(ServerDownEvent, src.ID(), serverDownErr)
		}
	}
}

func (c *HttpClient) raiseRequestEvent(srv *balancer.Server, err error) {
	if c.eventHandler != nil {
		src := srv.UserData().(*Source)
		if err == nil {
			c.eventHandler(RequestSuceededEvent, src.ID(), nil)
		} else {
			c.eventHandler(RequestFailedEvent, src.ID(), err)
		}
	}
}
