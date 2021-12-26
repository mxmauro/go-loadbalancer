package loadbalancer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------

const (
	serverOneCount = 5
	serverOneName  = "server 1"

	serverTwoCount = 2
	serverTwoName  = "server 2"

	backupServerName = "backup server"

	serverTotalCount = serverOneCount + serverTwoCount
)

// -----------------------------------------------------------------------------

func TestNoFail(t *testing.T) {
	lb := createTestLoadBalancer(false)

	for idx := 0; idx < serverTotalCount*2; idx++ {
		srv := lb.Next()

		srvName, _ := srv.UserData().(string)
		if (idx % serverTotalCount) < serverOneCount {
			assert.Equal(t, serverOneName, srvName)
		} else {
			assert.Equal(t, serverTwoName, srvName)
		}

		srv.SetOnline()
	}
}

func TestFailAll(t *testing.T) {
	lb := createTestLoadBalancer(false)

	for idx := 0; idx < 6; idx++ {
		srv := lb.Next()

		srv.SetOffline()
	}

	// At this point next server should be none
	srv := lb.Next()
	assert.Equal(t, (*Server)(nil), srv)
}

func TestBackup(t *testing.T) {
	lb := createTestLoadBalancer(true)

	for idx := 0; idx < 6; idx++ {
		srv := lb.Next()

		srv.SetOffline()
	}

	// At this point next server should be the backup one
	srv := lb.Next()

	srvName, _ := srv.UserData().(string)
	assert.Equal(t, backupServerName, srvName)

	srv.SetOffline() // NOTE: This call will act as a NO-OP
}

func TestWait(t *testing.T) {
	lb := createTestLoadBalancer(false)

	for idx := 0; idx < 6; idx++ {
		srv := lb.Next()

		srv.SetOffline()
	}

	// At this point next server should be none
	srv := lb.Next()
	assert.Equal(t, (*Server)(nil), srv)

	// Wait until a server becomes available (after ~1sec)
	ch := lb.WaitNext()
	srv = <-ch

	// At this point server 2 should be online again
	srvName, _ := srv.UserData().(string)
	assert.Equal(t, srvName, serverTwoName)
}

// -----------------------------------------------------------------------------
// Private functions

func createTestLoadBalancer(addBackup bool) *LoadBalancer {
	lb := Create()

	_ = lb.Add(ServerOptions{
		Weight:      serverOneCount,
		MaxFails:    3,
		FailTimeout: 5 * time.Second,
	}, serverOneName)

	_ = lb.Add(ServerOptions{
		Weight:      serverTwoCount,
		MaxFails:    3,
		FailTimeout: 1 * time.Second,
	}, serverTwoName)

	if addBackup {
		_ = lb.Add(ServerOptions{
			IsBackup: true,
		}, backupServerName)
	}

	return lb
}
