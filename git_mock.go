package gpoll

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type gitMock struct {
	baseMock
}

func (g *gitMock) HeadHash(repo *git.Repository) (string, error) {
	args := g.Called(repo)
	return args.String(0), args.Error(1)
}

func (g *gitMock) Clone(remote, branch, directory string) (*git.Repository, error) {
	args := g.Called(remote, branch, directory)
	return g.gitRepository(args, 0), args.Error(1)
}

func (g *gitMock) DiffRemote(repo *git.Repository, branch string) ([]GitChange, error) {
	args := g.Called(repo, branch)
	return g.gitChangeSlice(args, 0), args.Error(1)
}

func (g *gitMock) FetchLatestRemoteCommit(repo *git.Repository, branch string) (*object.Commit, error) {
	args := g.Called(repo, branch)
	return g.gitCommit(args, 0), args.Error(1)
}
