// See the LICENSE file for license details.

package loadbalancer

import (
	"time"
)

// -----------------------------------------------------------------------------

// Server represents an upstream server in a load balancer.
type Server struct {
	lb          *LoadBalancer // NOTE: Go's Mark & Sweep plays well with this circular reference
	opts        ServerOptions
	index       int
	isDown      bool
	failCounter int
	// NOTE: failTimestamp has two uses:
	//       1. Marks the timestamp of the first access failure
	//       2. Marks the timestamp to put it again online when down
	failTimestamp time.Time
	userData      interface{}
}

// ServerOptions specifies the weight, fail timeout and other options of a server.
type ServerOptions struct {
	// Weight
	Weight int

	// Maximum amount of unsuccessful attempts to reach the server that must happen in the time frame specified by the
	// FailTimeout parameter before setting it offline. The FailTimeout must be also specified. A value of zero
	// means the server will never go offline.
	MaxFails int

	// Fail timeout sets the time period where MaxFails unsuccessful attempts must happen in order to set a server
	// offline. Once the server becomes offline, MaxFails indicates how much time should pass before putting the server
	// online again.
	FailTimeout time.Duration

	// Indicates if this server must be used as a backup fail over. Backup servers never goes offline.
	IsBackup bool
}

// ServerGroup is a group of servers. Used to classify and track primary and backup servers.
type ServerGroup struct {
	srvList          []Server
	currServerIdx    int
	currServerWeight int
}

// -----------------------------------------------------------------------------

// UserData returns the server user data
func (srv *Server) UserData() interface{} {
	return srv.userData
}

// SetOnline marks a server as available
func (srv *Server) SetOnline() {
	// We only can change the online/offline status on primary servers
	if srv.opts.MaxFails == 0 || srv.opts.IsBackup {
		return
	}

	notifyUp := false

	// Lock access
	srv.lb.mtx.Lock()

	// Reset the failure counter
	srv.failCounter = 0

	// If the server was marked as down, put it online again
	if srv.isDown {
		srv.isDown = false
		srv.lb.primaryOnlineCount += 1

		notifyUp = true
	}

	// Unlock access
	srv.lb.mtx.Unlock()

	// Call event callback
	if notifyUp {
		srv.lb.raiseEvent(ServerUpEvent, srv)
	}
}

// SetOffline marks a server as unavailable
func (srv *Server) SetOffline() {
	// We only can change the online/offline status on primary servers
	if srv.opts.MaxFails == 0 || srv.opts.IsBackup {
		return
	}

	notifyDown := false

	// Lock access
	srv.lb.mtx.Lock()

	// If server is up
	if !srv.isDown && srv.failCounter < srv.opts.MaxFails {
		now := time.Now()

		// Increment the failure counter
		srv.failCounter += 1

		if srv.failCounter == 1 {
			// If it is the first failure, set the fail timestamp limit
			srv.failTimestamp = now.Add(srv.opts.FailTimeout)

		} else {

			// If this failure passed after the fail timeout, reset the counter
			if now.After(srv.failTimestamp) {
				srv.failCounter = 1
			}
		}

		// If we reach to the maximum failure count, put this server offline
		if srv.failCounter == srv.opts.MaxFails {
			srv.isDown = true
			srv.failTimestamp = now.Add(srv.opts.FailTimeout)
			srv.lb.primaryOnlineCount -= 1

			notifyDown = true
		}
	}

	// Unlock access
	srv.lb.mtx.Unlock()

	// Call event callback
	if notifyDown {
		srv.lb.raiseEvent(ServerDownEvent, srv)
	}
}
