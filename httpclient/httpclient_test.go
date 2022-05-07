package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randlabs/go-loadbalancer"
	"github.com/randlabs/go-loadbalancer/httpclient"
)

// -----------------------------------------------------------------------------

type MockServer struct {
	srv *httptest.Server
	simulateDown int32
}

// -----------------------------------------------------------------------------

func TestHttpClient(t *testing.T) {
	// Create mock servers and http client requester
	server1, server2, hc := createTestEnvironment(t)
	defer server1.Destroy()
	defer server2.Destroy()

	// We have to get the correct response from each server
	req := hc.NewRequest("GET", "/test")
	err := req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}
		if res.Header.Get("x-server") != "server1" {
			return errors.New("expected server to be `server1`")
		}

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	req = hc.NewRequest("GET", "/test")
	err = req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}
		if res.Header.Get("x-server") != "server2" {
			return errors.New("expected server to be `server2`")
		}

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestHttpClientFailFirst(t *testing.T) {
	// Create mock servers and http client requester
	server1, server2, hc := createTestEnvironment(t)
	defer server1.Destroy()
	defer server2.Destroy()

	// Do a request and assume it is not up-to-date, so we put it offline
	req := hc.NewRequest("GET", "/test")
	err := req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}
		if res.Header.Get("x-server") != "server1" {
			return errors.New("expected server to be `server1`")
		}

		// Set this server offline
		res.SetOffline()

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	// Now we have to get a response from the second server
	req = hc.NewRequest("GET", "/test")
	err = req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}
		if res.Header.Get("x-server") != "server2" {
			return errors.New("expected server to be `server2`")
		}

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	// Because the first server is offline, again we have to get a response from the second server
	req = hc.NewRequest("GET", "/test")
	err = req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}
		if res.Header.Get("x-server") != "server2" {
			return errors.New("expected server to be `server2`")
		}

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestHttpClientPostRetry(t *testing.T) {
	// Create mock servers and http client requester
	server1, server2, hc := createTestEnvironment(t)
	defer server1.Destroy()
	defer server2.Destroy()

	// Do a request and assume it is not up-to-date, so we put it offline
	req := hc.NewRequest("POST", "/bodytest")
	req.SetBodyBytes([]byte("this is a sample body"))
	err := req.Exec(context.Background(), func (ctx context.Context, res httpclient.Response) error {
		if res.StatusCode != 200 {
			return fmt.Errorf("unexpected status code %v", res.StatusCode)
		}

		retryCount := res.RetryCount()
		switch retryCount {
		case 0:
			fallthrough
		case 2:
			if res.Header.Get("x-server") != "server1" {
				return errors.New("expected server to be `server1`")
			}

			// Retry on the next available server
			res.RetryOnNextServer()

		case 1:
			fallthrough
		case 3:
			if res.Header.Get("x-server") != "server2" {
				return errors.New("expected server to be `server2`")
			}

			// Retry on the next available server
			res.RetryOnNextServer()

		case 4:
			// When we hit the fourth retry, check if the body was received correctly
			// by inspecting the expected response.
			m := make(map[string]interface{})
			err := json.NewDecoder(res.Body).Decode(&m)
			if err != nil {
				return err
			}
			body, ok := m["received-body"]
			if !ok {
				return errors.New("received-body not present")
			}
			if body.(string) != "this is a sample body" {
				return errors.New("received-body mismatch")
			}
		}

		// Done
		return nil
	})
	if err != nil {
		t.Fatal(err.Error())
	}
}

// -----------------------------------------------------------------------------

func createTestEnvironment(t *testing.T) (*MockServer, *MockServer, *httpclient.HttpClient) {
	// Create two mock servers and our load-balanced http client
	server1 := createMockTimestampServer("server1")
	server2 := createMockTimestampServer("server2")

	hc := httpclient.Create()
	err := hc.AddSource(
		server1.URL(),
		map[string][]string{
			"x-expected-server": { "server1" },
		},
		loadbalancer.ServerOptions{
			Weight:   1,
			MaxFails: 1,
			FailTimeout: 10 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("unable to add source to load balancer [err=%v]", err.Error())
	}

	err = hc.AddSource(
		server2.URL(),
		map[string][]string{
			"x-expected-server": { "server2" },
		},
		loadbalancer.ServerOptions{
			Weight:   1,
			MaxFails: 1,
			FailTimeout: 10 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("unable to add source to load balancer [err=%v]", err.Error())
	}

	// Done
	return server1, server2, hc
}

func createMockTimestampServer(serverName string) *MockServer {
	ms := MockServer{}

	// Create a new mock server with a simple endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-server", serverName)

		if atomic.LoadInt32(&ms.simulateDown) != 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service unavailable"))
			return
		}

		switch r.Method {
		case "GET":
			if r.URL.Path == "/test" {
				resp := make(map[string]interface{})
				resp["timestamp"] = time.Now().Format("2006-01-02 15:04:05")

				s := r.Header.Get("x-sample")
				if len(s) > 0 {
					resp["received-x-sample"] = s
				}
				resp["server-match"] = r.Header.Get("x-expected-server") != serverName

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

		case "POST":
			if r.URL.Path == "/bodytest" && r.Body != nil {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("error: " + err.Error()))
					return
				}

				resp := make(map[string]interface{})
				resp["received-body"] = string(body)

				s := r.Header.Get("x-sample")
				if len(s) > 0 {
					resp["received-x-sample"] = s
				}
				resp["server-match"] = r.Header.Get("x-expected-server") != serverName

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}

		// If we reach here, we have a bad request
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	ms.srv = srv
	// Done
	return &ms
}

func (ms *MockServer) Destroy()  {
	ms.srv.Close()
}

func (ms *MockServer) URL() string {
	return ms.srv.URL
}

func (ms *MockServer) SetOffline(offline bool) {
	if offline {
		_ = atomic.SwapInt32(&ms.simulateDown, 1)
	} else {
		_ = atomic.SwapInt32(&ms.simulateDown, 0)
	}
}
