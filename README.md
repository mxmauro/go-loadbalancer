# go-loadbalancer

A round-robin server selection (aka load balancer) library.

The base code of this library, the balancer, *DOES NOT DO* any kind of network access. It's goal is to automatically select an upstream handler in a set of primary and backup configured servers.

## Usage with example

```golang
import (
    "time"

    balancer "github.com/randlabs/go-loadbalancer/v2"
)

type ServerInfo struct {
    Name string
    URL string
}

func main() {
    // Create a new load balancer
    lb := balancer.Create()

    // Add a dummy server, the second parameter should contain some information to
    // assist the developer to identify the server details. In our example we will
    // store some server-related details
    info := &ServerInfo{
        Name: "Server A",
        URL: "https://10.0.0.1"
    }
    err := lb.Add(lb.ServerOptions{
        // Sets the server weight, defaults to 1
        Weight:      5,
        // Sets the number of unsuccessful attempts that should happen in the duration
        // set by the fail_timeout parameter to consider the server unavailable for a
        // duration also set by the fail_timeout parameter
        MaxFails:    10,
        // Also, when a server becomes unavailable, FailTimeout sets the time to wait
        // to consider the server as available again
        FailTimeout: 10 * time.Seconds,
        // A backup is a set of servers to use when all the primary ones becomes
        // unavailable. Backup servers ignores MaxFails and FailTimeout parameters
        IsBackup:    false,
    }, info)

    // Add a second server
    info = &ServerInfo{
        Name: "Server B",
        URL: "https://10.0.0.2"
    }
    err = lb.Add(lb.ServerOptions{
        Weight:      1,
        MaxFails:    10,
        FailTimeout: 10 * time.Seconds,
        IsBackup:    false,
    }, info)

    // Get the next server to use. Next can return nil if all primary servers are not
    // available and no backup servers. In this case, we recommend to wait until a
    // server becomes available again.
    srv := lb.Next()

    info, _ = srv.UserData().(*ServerInfo)

    // ...
    // Execute the request to the selected server using info.URL
    // ...

    // Inform the load balancer manager if the access to that server was successful
    // or not
    if requestSucceeded {
        srv.SetOnline()
    } else {
        srv.SetOffline()
    }

    // We can also use channels to wait until a server becomes available
    ch := lb.WaitNext()
    srv = <-ch
}
```

## httpclient

The `httpclient` module, implements an alternative to `http.Client` that allows to use a set of servers and balance
requests among them.

> Most load-balanced http client libraries makes use of the `RoundTripper` interface, but we don't.
>
> The major reason for this is we want to allow the dev, to be able to mark a server (temporarily) offline or retry
> the operation, not only if connection to the server is established, but also depending on the response.
>
> For e.g., let's say your backend correctly answers a request but the output indicates the internal processing is not
> up-to-date, then you can decide to stop using that server until it is.

### Usage:

```golang
import (
    "fmt"

    balancer "github.com/randlabs/go-loadbalancer/v2"
)

func main() {
    hc := httpclient.Create()
    _ = hc.AddSource("https://server1.test-network", httpclient.SourceOptions{
        ServerOptions: httpclient.ServerOptions{
            Weight:      1,
            MaxFails:    1,
            FailTimeout: 10 * time.Second,
        },
    })
    _ = hc.AddSource("https://server2.test-network", httpclient.SourceOptions{
        ServerOptions: httpclient.ServerOptions{
            Weight:      1,
            MaxFails:    1,
            FailTimeout: 10 * time.Second,
        },
    })

    err := hc.NewRequest(context.Background(), "/api-test").
        Method("GET").
        Callback(func (ctx context.Context, res httpclient.Response) error {
            if res.Err() != nil || res.StatusCode != 200 {
                // Retry on the next available server on failed request
                res.RetryOnNextServer()
                return nil
            }

            // Process response
            // ...

            // Done
            return nil
        }).
        Exec()
}
```

## License

See [LICENSE](/LICENSE) file for details.

