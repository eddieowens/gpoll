package tests

import (
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
			Remote: "git@github.com:eddieowens/gpoll.git",
		},
		Interval: 1,
	})

	if !g.NoError(err) {
		g.FailNow(err.Error())
	}

	// -- When
	//

	// -- Then
	//
}

func TestGpoll(t *testing.T) {
	suite.Run(t, new(Gpoll))
}
