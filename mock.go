package gpoll

import (
	"github.com/stretchr/testify/mock"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type baseMock struct {
	mock.Mock
}

func (b *baseMock) gitRepository(args mock.Arguments, i int) *git.Repository {
	var r *git.Repository
	v := args.Get(i)
	if v != nil {
		r = v.(*git.Repository)
	}
	return r
}

func (b *baseMock) gitChangeSlice(args mock.Arguments, i int) []FileChange {
	var r []FileChange
	v := args.Get(i)
	if v != nil {
		r = v.([]FileChange)
	}
	return r
}

func (b *baseMock) gitCommit(args mock.Arguments, i int) *object.Commit {
	var r *object.Commit
	v := args.Get(i)
	if v != nil {
		r = v.(*object.Commit)
	}
	return r
}
