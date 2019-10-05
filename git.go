package gpoll

import (
	"errors"
	"fmt"
	"gopkg.in/src-d/go-billy.v4/memfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	"gopkg.in/src-d/go-git.v4/utils/merkletrie"
	"time"
)

// Represents a change to a file within the target Git repo.
type FileChange struct {
	// The name and absolute path to the changed file.
	Filepath string

	// The type of change that occurred e.g. added, created, deleted the file.
	ChangeType ChangeType
}

// Represents a batch of changes to files between two commits in a Git repo.
type CommitDiff struct {
	// The list of changes that occurred in the commit.
	Changes []FileChange

	// The base for the file changes.
	From Commit

	// The result of the file changes.
	To Commit
}

type Commit struct {
	// The Sha of the commit.
	Sha string

	// When the commit occurred in UTC.
	When time.Time

	// The author of the commit.
	Author Author

	// The message made by the author.
	Message string
}

type Author struct {
	Name string

	Email string
}

type ChangeType int

const (
	// A pre-existing file was edited in the commit.
	ChangeTypeUpdate ChangeType = iota

	// A new file was created in the commit.
	ChangeTypeCreate

	// A pre-existing file was deleted in the commit.
	ChangeTypeDelete

	// The file is present from the initial clone of the repo. Only ever used once for the clone of the repo.
	ChangeTypeInit
)

const remoteName = "origin"

func newGit(config GitConfig) (GitService, error) {
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

type GitAuthConfig struct {
	// The filepath to the SSH key. Required if the Username and Password are not set.
	SshKey string `validation:"required_without=Username Password"`

	// The username for the git repo. Required if the SshKey is not set or if the Password is set.
	Username string `validation:"required_without=SshKey,required_with=Password"`

	// The password for the git repo. Required if the SshKey is not set or if the Username is set.
	Password string `validation:"require_without=SshKey,required_with=Username"`
}

type GitService interface {
	Clone(remote, branch, directory string) (*git.Repository, error)
	DiffRemote(repo *git.Repository, branch string) ([]CommitDiff, error)
	FetchLatestRemoteCommit(repo *git.Repository, branch string) (*object.Commit, error)
	HeadCommit(repo *git.Repository) (*object.Commit, error)
	Diff(from *object.Commit, to *object.Commit) (*CommitDiff, error)
	ToInternal(c *object.Commit) *Commit
}

type gitImpl struct {
	authMethod transport.AuthMethod
}

func (g *gitImpl) ToInternal(c *object.Commit) *Commit {
	return &Commit{
		Sha:  c.Hash.String(),
		When: c.Author.When.UTC(),
		Author: Author{
			Name:  c.Author.Name,
			Email: c.Author.Email,
		},
		Message: c.Message,
	}
}

func (g *gitImpl) Diff(from *object.Commit, to *object.Commit) (*CommitDiff, error) {
	toTree, err := to.Tree()
	if err != nil {
		return nil, err
	}
	fromTree, err := from.Tree()
	if err != nil {
		return nil, err
	}

	diffs, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, err
	}

	changes := make([]FileChange, 0)
	for _, d := range diffs {
		a, err := d.Action()
		if err != nil {
			return nil, err
		}

		gitChange := FileChange{}
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

		changes = append(changes, gitChange)
	}

	return &CommitDiff{
		Changes: changes,
		From:    *g.ToInternal(from),
		To:      *g.ToInternal(to),
	}, nil
}

func (g *gitImpl) HeadCommit(repo *git.Repository) (*object.Commit, error) {
	h, err := repo.Head()
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(h.Hash())
}

func (g *gitImpl) DiffRemote(repo *git.Repository, branch string) ([]CommitDiff, error) {
	err := repo.Fetch(&git.FetchOptions{
		Auth: g.authMethod,
	})
	if err != nil {
		if err != git.NoErrAlreadyUpToDate {
			return nil, err
		}
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

	commits, err := g.listCommits(currentCommit, remCommit)
	if err != nil {
		return nil, err
	}

	from := currentCommit
	diffs := make([]CommitDiff, len(commits)-1)
	for i := 1; i < len(commits); i++ {
		to := commits[i]
		diff, err := g.Diff(from, to)
		if err != nil {
			return nil, err
		}
		diffs[i-1] = *diff
		from = to
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

	return diffs, nil
}

func (g *gitImpl) Clone(remote, branch, directory string) (*git.Repository, error) {
	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
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

func (g *gitImpl) listCommits(from *object.Commit, to *object.Commit) ([]*object.Commit, error) {
	var err error
	parent := to
	cs := make([]*object.Commit, 0)
	// Get all commits working backwards from the "to" commit
	for err == nil && parent.Hash != from.Hash {
		cs = append(cs, parent)
		parent, err = parent.Parents().Next()
	}
	if err != nil {
		return nil, err
	}
	cs = append(cs, from)

	// Reverse the order
	l := len(cs)
	commits := make([]*object.Commit, l)
	for i := range commits {
		commits[i] = cs[l-(i+1)]
	}

	return commits, nil
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
