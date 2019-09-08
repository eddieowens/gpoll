package gpoll

import (
	"errors"
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/utils/merkletrie"
)

// Represents a change to a file within the target Git repo.
type GitChange struct {
	// The name and absolute path to the changed file.
	Filepath string

	// The commit sha associated with the change.
	Sha string

	// The type of change that occurred e.g. added, created, deleted the file.
	ChangeType ChangeType
}

type ChangeType int

const (
	ChangeTypeUpdate ChangeType = iota
	ChangeTypeCreate
	ChangeTypeDelete
)

const remoteName = "origin"

func newGit(config GitConfig) (gitService, error) {
	auth, err := toAuthMethod(&config.Auth)
	if err != nil {
		return nil, err
	}
	return &gitImpl{
		authMethod: auth,
	}, nil
}

type GitConfig struct {
	// Authentication/authorization for the git repo to poll. Required.
	Auth GitAuthConfig `validate:"required"`

	// The remote git repository that should be polled. Required.
	Remote string `validate:"required"`

	// The branch of the git repo to poll. Defaults to master.
	Branch string

	// The directory that the git repository will be cloned into. Defaults to the current directory.
	CloneDirectory string
}

type gitService interface {
	Clone(remote, branch, directory string) (*git.Repository, error)
	DiffRemote(repo *git.Repository, branch string) ([]GitChange, error)
	FetchLatestRemoteCommit(repo *git.Repository, branch string) (*object.Commit, error)
}

type gitImpl struct {
	authMethod transport.AuthMethod
}

func (g *gitImpl) DiffRemote(repo *git.Repository, branch string) ([]GitChange, error) {
	err := repo.Fetch(&git.FetchOptions{
		Auth: g.authMethod,
	})
	if err != nil {
		return nil, err
	}

	h, err := repo.Head()
	if err != nil {
		return nil, err
	}

	remCommit, err := g.FetchLatestRemoteCommit(repo, branch)
	if err != nil {
		return nil, err
	}

	currentCommit, err := repo.CommitObject(h.Hash())
	if err != nil {
		return nil, err
	}

	originTree, err := remCommit.Tree()
	if err != nil {
		return nil, err
	}
	branchTree, err := currentCommit.Tree()
	if err != nil {
		return nil, err
	}

	diffs, err := branchTree.Diff(originTree)
	if err != nil {
		return nil, err
	}

	changes := make([]GitChange, 0)
	for _, d := range diffs {
		a, err := d.Action()
		if err != nil {
			return nil, err
		}

		gitChange := GitChange{}
		switch a {
		case merkletrie.Modify:
			gitChange.ChangeType = ChangeTypeUpdate
		case merkletrie.Delete:
			gitChange.ChangeType = ChangeTypeDelete
		case merkletrie.Insert:
			gitChange.ChangeType = ChangeTypeCreate
		}

		if gitChange.ChangeType == ChangeTypeDelete {
			gitChange.Filepath = d.From.Name
		} else {
			gitChange.Filepath = d.To.Name
		}
		gitChange.Sha = remCommit.Hash.String()

		changes = append(changes, gitChange)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	err = wt.Pull(&git.PullOptions{
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          g.authMethod,
	})

	if err != nil {
		return nil, err
	}

	return changes, nil
}

func (g *gitImpl) Clone(remote, branch, directory string) (*git.Repository, error) {
	repo, err := git.PlainClone(directory, false, &git.CloneOptions{
		URL:           remote,
		RemoteName:    remoteName,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          g.authMethod,
	})

	if err == git.ErrRepositoryAlreadyExists {
		return git.PlainOpen(directory)
	} else if err != nil {
		return nil, err
	}

	return repo, nil
}

func (g *gitImpl) FetchLatestRemoteCommit(repo *git.Repository, branch string) (*object.Commit, error) {
	rem, err := repo.Remote(remoteName)
	if err != nil {
		return nil, err
	}

	rfs, err := rem.List(&git.ListOptions{
		Auth: g.authMethod,
	})
	if err != nil {
		return nil, err
	}

	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	for _, v := range rfs {
		if v.Name().String() == branchRef {
			c, err := repo.CommitObject(v.Hash())
			if err != nil {
				return nil, err
			}
			return c, nil
		}
	}
	return nil, errors.New("commit for ref could not be found")
}
