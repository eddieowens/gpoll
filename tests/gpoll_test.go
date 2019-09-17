package tests

import (
	"fmt"
	"github.com/eddieowens/gpoll"
	"github.com/stretchr/testify/suite"
	"testing"
)

type Gpoll struct {
	suite.Suite
}

func (g *Gpoll) SetupTest() {
}

func (g *Gpoll) Test() {
	// -- Given
	//
	poller, err := gpoll.NewPoller(gpoll.PollConfig{
		Git: gpoll.GitConfig{
			Auth: gpoll.GitAuthConfig{
				SshKey: "~/.ssh/id_rsa",
			},
			Remote:         "git@github.com:eddieowens/gpoll.git",
			Branch:         "test/something",
			CloneDirectory: "./something",
		},
		HandleChange: func(change gpoll.GitChange) {
			fmt.Println(change)
		},
	})

	if !g.NoError(err) {
		g.FailNow(err.Error())
	}

	// -- When
	//
	g.NoError(poller.Start())

	// -- Then
	//
}

func TestGpoll(t *testing.T) {
	suite.Run(t, new(Gpoll))
}
