package loadbalancer

import (
	"errors"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------

type LoadBalancer struct {
	mtx                sync.Mutex
	primaryGroup       ServerGroup
	backupGroup        ServerGroup
	primaryOnlineCount int
}

const (
	InvalidParamsErr = "invalid parameter"
)

// -----------------------------------------------------------------------------

// Create creates a new load balancer manager
func Create() *LoadBalancer {
	lb := LoadBalancer{
		mtx: sync.Mutex{},
		primaryGroup: ServerGroup{
			srvList: make([]Server, 0),
		},
		backupGroup: ServerGroup{
			srvList: make([]Server, 0),
		},
	}
	return &lb
}

// Add adds a new server to the list
func (lb *LoadBalancer) Add(opts ServerOptions, userData interface{}) error {
	// Check options
	if opts.Weight < 0 {
		return errors.New(InvalidParamsErr)
	}
	if !opts.IsBackup {
		if opts.MaxFails < 0 || opts.FailTimeout <= time.Duration(0) {
			return errors.New(InvalidParamsErr)
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
	if opts.IsBackup {
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
	now := time.Now()

	// Lock access
	lb.mtx.Lock()
	defer lb.mtx.Unlock()

	// If all primary servers are offline, check if we can put someone up
	if lb.primaryOnlineCount == 0 {
		for idx := range lb.primaryGroup.srvList {
			if now.After(lb.primaryGroup.srvList[idx].failTimestamp) {
				// Put this server online again
				lb.primaryGroup.srvList[idx].isDown = false
				lb.primaryGroup.srvList[idx].failCounter = 0

				lb.primaryOnlineCount += 1
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
			}

			if !srv.isDown && lb.primaryGroup.currServerWeight < srv.opts.Weight {
				// Got a server!
				lb.primaryGroup.currServerWeight += 1

				// Done
				return srv
			}

			// Advance to next server
			lb.primaryGroup.currServerIdx += 1
			if lb.primaryGroup.currServerIdx >= len(lb.primaryGroup.srvList) {
				lb.primaryGroup.currServerIdx = 0
			}

			lb.primaryGroup.currServerWeight = 0
		}
	}

	// If we reach here, there is no primary server available
	if len(lb.backupGroup.srvList) > 0 {
		for {
			srv := &lb.backupGroup.srvList[lb.backupGroup.currServerIdx]

			if lb.backupGroup.currServerWeight < srv.opts.Weight {
				// Got a server!
				lb.backupGroup.currServerWeight += 1

				// Done
				return srv
			}

			// Advance to next server
			lb.backupGroup.currServerIdx += 1
			if lb.backupGroup.currServerIdx >= len(lb.backupGroup.srvList) {
				lb.backupGroup.currServerIdx = 0
			}

			lb.backupGroup.currServerWeight = 0
		}
	}

	// No available server
	return nil
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
