# go-loadbalancer

A round-robin server selection (aka load balancer) library.

**IMPORTANT NOTE**: This library DOES NOT DO any kind of networking. It aims to automatically select an upstream handler in a set of primary and backup configured servers.

## Usage with example

```golang
import (
    "time"

    balancer "github.com/randlabs/go-loadbalancer"
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
        // Sets the number of unsuccessful attempts that should happen in the duration set by the fail_timeout parameter to consider the server unavailable for a duration also set by the fail_timeout parameter
        MaxFails:    10,
        FailTimeout: 10 * time.Seconds,
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

    // Inform the load balancer manager if the access to that server was successful or not
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

## Lincese
See `LICENSE` file for details.
