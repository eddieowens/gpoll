package gpoll

import (
	"github.com/bxcodec/faker/v3"
	"github.com/eddieowens/gpoll/mocks"
	"github.com/stretchr/testify/suite"
	"gopkg.in/src-d/go-git.v4"
	"testing"
)

type GpollTest struct {
	suite.Suite

	gitMock *mocks.GitService
	p       *poller
}

func (g *GpollTest) SetupTest() {
	g.gitMock = new(mocks.GitService)
	p, err := NewPoller(PollConfig{
		Git: GitConfig{
			Auth: GitAuthConfig{
				Username: faker.Username(),
				Password: faker.Username(),
			},
			Remote: faker.Username(),
		},
		Interval: 1,
	})
	if !g.NoError(err) {
		g.FailNow(err.Error())
	}

	g.p = p.(*poller)
	g.p.git = g.gitMock
}

func (g *GpollTest) TestStart() {
	// -- Given
	//repo *git.Repository, branch string
	remote := g.p.config.Git.Remote
	branch := g.p.config.Git.Branch
	directory := g.p.config.Git.CloneDirectory
	repo := new(git.Repository)

	changes := FakeGitChanges()

	g.gitMock.On("Clone", remote, branch, directory).Return(repo, nil)
	g.gitMock.On("DiffRemote", repo, branch).Return(changes, nil)

	// -- When
	//
	c, err := g.p.StartAsync()

	// -- Then
	//
	if g.NoError(err) {
		n := len(changes)
		for i := 0; i < n; i++ {
			g.Contains(changes, <-c)
		}
	}
}

func RandInt(l, u int) int {
	is, _ := faker.RandomInt(l, u-1)
	return is[0]
}

func FakeGitChanges() []FileChange {
	c := RandInt(0, 3)
	n := RandInt(1, 10)
	cs := make([]FileChange, n)
	for i := range cs {
		cs[i] = FileChange{
			Filepath:   faker.Username(),
			ChangeType: ChangeType(c),
			Sha:        faker.Username(),
		}
	}
	return cs
}

func TestGpollTest(t *testing.T) {
	suite.Run(t, new(GpollTest))
}
