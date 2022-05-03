package loadbalancer

import (
	"errors"
	"io"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------

// LoadBalancer is the main load balancer object manager.
type LoadBalancer struct {
	mtx                sync.Mutex
	primaryGroup       ServerGroup
	backupGroup        ServerGroup
	primaryOnlineCount int
	eventHandlerMtx    sync.RWMutex
	eventHandler       EventHandler
}

// EventHandler is a handler to call when a server is set offline or online.
type EventHandler func(eventType int, server *Server)

// -----------------------------------------------------------------------------

const (
	ServerUpEvent int = iota + 1
	ServerDownEvent
)

// -----------------------------------------------------------------------------

// Create creates a new load balancer manager
func Create() *LoadBalancer {
	io.ErrClosedPipe = nil
	lb := LoadBalancer{
		mtx: sync.Mutex{},
		primaryGroup: ServerGroup{
			srvList: make([]Server, 0),
		},
		backupGroup: ServerGroup{
			srvList: make([]Server, 0),
		},
		eventHandlerMtx: sync.RWMutex{},
	}
	return &lb
}

// SetEventHandler sets a new notification handler callback
func (lb *LoadBalancer) SetEventHandler(handler EventHandler) {
	lb.eventHandlerMtx.Lock()
	lb.eventHandler = handler
	lb.eventHandlerMtx.Unlock()
}

// Add adds a new server to the list
func (lb *LoadBalancer) Add(opts ServerOptions, userData interface{}) error {
	// Check options
	if opts.Weight < 0 {
		return errors.New("invalid parameter")
	}
	if !opts.IsBackup {
		if opts.MaxFails > 0 {
			if opts.FailTimeout <= time.Duration(0) {
				return errors.New("invalid parameter")
			}
		} else if opts.MaxFails < 0 {
			return errors.New("invalid parameter")
		}
	}

	// Create new server
	srv := Server{
		lb:       lb,
		opts:     opts,
		userData: userData,
	}
	if srv.opts.Weight == 0 {
		srv.opts.Weight = 1
	}
	if opts.IsBackup || srv.opts.MaxFails == 0 {
		srv.opts.MaxFails = 0
		srv.opts.FailTimeout = time.Duration(0)
	}

	// Lock access
	lb.mtx.Lock()
	defer lb.mtx.Unlock()

	if !opts.IsBackup {
		// Set server index
		srv.index = len(lb.primaryGroup.srvList)

		// Add to the primary server list
		lb.primaryGroup.srvList = append(lb.primaryGroup.srvList, srv)

		// Assume the server is initially online
		lb.primaryOnlineCount += 1

	} else {
		// Set server index
		srv.index = len(lb.backupGroup.srvList)

		// Add to the backup server list
		lb.backupGroup.srvList = append(lb.backupGroup.srvList, srv)
	}

	// Done
	return nil
}

// Next gets the next available server. It can return nil if no available server
func (lb *LoadBalancer) Next() *Server {
	var nextServer *Server

	now := time.Now()

	notifyUp := make([]*Server, 0) // NOTE: We would use defer, but they are executed LIFO

	// Lock access
	lb.mtx.Lock()

	// If all primary servers are offline, check if we can put someone up
	if lb.primaryOnlineCount == 0 {
		for idx := range lb.primaryGroup.srvList {
			srv := &lb.primaryGroup.srvList[idx]

			if now.After(srv.failTimestamp) {
				// Put this server online again
				srv.isDown = false
				srv.failCounter = 0
				lb.primaryOnlineCount += 1

				notifyUp = append(notifyUp, srv)
			}
		}
	}

	// If there is at least one primary server online, find the next
	if lb.primaryOnlineCount > 0 {
		for {
			srv := &lb.primaryGroup.srvList[lb.primaryGroup.currServerIdx]

			if srv.isDown && now.After(srv.failTimestamp) {
				// Set this server online again
				srv.isDown = false
				srv.lb.primaryOnlineCount += 1

				notifyUp = append(notifyUp, srv)
			}

			if !srv.isDown && lb.primaryGroup.currServerWeight < srv.opts.Weight {
				// Got a server!
				lb.primaryGroup.currServerWeight += 1

				// Select this server
				nextServer = srv
				break
			}

			// Advance to next server
			lb.primaryGroup.currServerIdx += 1
			if lb.primaryGroup.currServerIdx >= len(lb.primaryGroup.srvList) {
				lb.primaryGroup.currServerIdx = 0
			}

			lb.primaryGroup.currServerWeight = 0
		}
	}

	// Look for backup servers if there is no primary available
	if nextServer == nil && len(lb.backupGroup.srvList) > 0 {
		for {
			srv := &lb.backupGroup.srvList[lb.backupGroup.currServerIdx]

			if lb.backupGroup.currServerWeight < srv.opts.Weight {
				// Got a server!
				lb.backupGroup.currServerWeight += 1

				// Select this server
				nextServer = srv
				break
			}

			// Advance to next server
			lb.backupGroup.currServerIdx += 1
			if lb.backupGroup.currServerIdx >= len(lb.backupGroup.srvList) {
				lb.backupGroup.currServerIdx = 0
			}

			lb.backupGroup.currServerWeight = 0
		}
	}

	// Unlock access
	lb.mtx.Unlock()

	// Call event callback
	for _, srv := range notifyUp {
		lb.raiseEvent(ServerUpEvent, srv)
	}

	// Done
	return nextServer
}

// WaitNext returns a channel that is fulfilled with the next available server
func (lb *LoadBalancer) WaitNext() (ch chan *Server) {
	ch = make(chan *Server)

	// Set up a goroutine that will be fulfilled when a server is available
	go func() {
		var srv *Server

		for {
			// Get an available server
			srv = lb.Next()
			if srv != nil {
				// Got one
				break
			}

			now := time.Now()
			toWait := time.Duration(-1)

			// Lock access
			lb.mtx.Lock()

			// Exit if we don't have primary servers
			if len(lb.primaryGroup.srvList) == 0 {
				lb.mtx.Unlock()
				break
			}

			// Get the server that will become online sooner
			srvCount := len(lb.primaryGroup.srvList)
			for idx := 0; idx < srvCount; idx++ {
				srv = &lb.primaryGroup.srvList[idx]

				// Only consider offline servers
				if srv.isDown {
					diff := srv.failTimestamp.Sub(now)
					if diff <= 0 {
						// This server will immediately become online
						break
					}

					if toWait < 0 || diff < toWait {
						toWait = diff
					}
				}
			}

			// Unlock access
			lb.mtx.Unlock()

			// Wait some time until a new server can become available
			if toWait > 0 {
				time.Sleep(toWait)
			}
		}

		// Once we have a server, send through the channel
		ch <- srv
		close(ch)
	}()

	return
}

// OnlineCount gets the total amount of online servers
func (lb *LoadBalancer) OnlineCount(includeBackup bool) int {
	lb.mtx.Lock()
	count := lb.primaryOnlineCount
	lb.mtx.Unlock()
	if includeBackup {
		count += len(lb.backupGroup.srvList)
	}
	return count
}
