# GPoll

[![GoDoc](https://godoc.org/github.com/eddieowens/gpoll?status.svg)](https://godoc.org/github.com/eddieowens/gpoll)

Go library for polling a Git repository.

## Installation
```
go get github.com/eddieowens/gpoll
```

## Usage
```go
package main

import (
    "fmt"
    "github.com/eddieowens/gpoll"
    "log"
)

func main() {
    poller, err := gpoll.NewPoller(gpoll.PollConfig{
        Git: gpoll.GitConfig{
            Auth: gpoll.GitAuthConfig{
                // Uses the SSH key from my local directory.
                SshKey: "~/.ssh/id_rsa",
            },
            // The target remote.
            Remote: "git@github.com:eddieowens/gpoll.git",
        },
        OnUpdate: func(change gpoll.GitChange) {
            switch change.EventType {
            case gpoll.EventTypeDelete:
                fmt.Printf("%s was deleted", change.Filename)
            case gpoll.EventTypeUpdate:
                fmt.Printf("%s was updated", change.Filename)
            case gpoll.EventTypeCreate:
                fmt.Printf("%s was created", change.Filename)
            }
        },
    })
    
    if err != nil {
        panic(err)
    }
    
    // Will poll the repo until poller.Stop() is called.
    log.Fatal(poller.Start())
}
```