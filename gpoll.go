// A library for polling a Git repository for changes.
package gpoll

import (
	"gopkg.in/go-playground/validator.v9"
	"gopkg.in/src-d/go-git.v4"
	"os"
	"path"
	"path/filepath"
	"time"
)

type Poller interface {
	// Start polling your git repo without blocking. The poller will diff the remote against the local clone directory at
	// the specified interval and return all changes through the configured callback and the returned channel.
	StartAsync() (chan Commit, error)

	// Start polling your git repo blocking whatever thread it is run on. The poller will diff the remote against the
	// local clone directory at the specified interval and return all changes through the configured callback.
	Start() error

	// Stop all polling.
	Stop()

	// Diff the remote and the local and return all differences.
	Poll() ([]Commit, error)
}

type HandleCommitFunc func(commit Commit)

type FileChangeFilterFunc func(change FileChange) bool

type PollConfig struct {
	Git GitConfig `validate:"required"`

	// Function for filtering out FileChanges made to a Git commit. If the function returns true, the FileChange will be
	// included in the commit passed into the HandleCommit calls. If false is returned, the file will always be ignored.
	FileChangeFilter FileChangeFilterFunc

	// Function that is called when a commit is made to the Git repo. This function maintains chronological order of
	// commits and is called synchronously.
	HandleCommit HandleCommitFunc

	// The polling interval. Defaults to 30 seconds.
	Interval time.Duration
}

// Create a new Poller from config. Will return an error for misconfiguration.
func NewPoller(config PollConfig) (Poller, error) {
	if config.Interval == 0 {
		config.Interval = 30 * time.Second
	}

	if config.Git.CloneDirectory == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.Git.CloneDirectory = wd
	}
	v := validator.New()
	if err := v.Struct(config); err != nil {
		return nil, err
	}

	g, err := newGit(config.Git)
	if err != nil {
		return nil, err
	}

	closer := make(chan bool, 1)
	onChangeChan := make(chan Commit, 1)

	poller := &poller{
		c:      onChangeChan,
		config: &config,
		closer: closer,
		git:    g,
	}

	return poller, nil
}

type poller struct {
	c      chan Commit
	config *PollConfig
	closer chan bool
	git    GitService
	repo   *git.Repository
}

func (p *poller) Start() error {
	ticker, err := p.setup()
	if err != nil {
		return err
	}

	p.loop(ticker)
	return nil
}

func (p *poller) StartAsync() (chan Commit, error) {
	ticker, err := p.setup()
	if err != nil {
		return nil, err
	}

	go p.loop(ticker)

	return p.c, nil
}

func (p *poller) Poll() ([]Commit, error) {
	changes, err := p.git.DiffRemote(p.repo, p.config.Git.Branch)
	if err != nil {
		return nil, err
	}

	if len(changes) > 0 {
		for _, change := range changes {
			for i, c := range change.Changes {
				if p.config.FileChangeFilter != nil {
					filteredChanges := make([]FileChange, 0)
					if p.config.FileChangeFilter(c) {
						filteredChanges = append(filteredChanges, c)
					}
					change.Changes = filteredChanges
				}
				change.Changes[i].Filepath = path.Join(p.config.Git.CloneDirectory, c.Filepath)
			}
		}
	}
	return changes, nil
}

func (p *poller) Stop() {
	p.closer <- true
}

func (p *poller) onStart() error {
	if p.config.HandleCommit == nil {
		return nil
	}
	commit, err := p.git.HeadCommit(p.repo)
	if err != nil {
		return err
	}
	gitDir := path.Join("*", ".git")
	changes := make([]FileChange, 0)
	err = filepath.Walk(p.config.Git.CloneDirectory, func(fp string, _ os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		isInGitDir, _ := filepath.Match(path.Join(gitDir, "*"), fp)
		isGitDir, _ := filepath.Match(gitDir, fp)
		if isInGitDir || isGitDir {
			return filepath.SkipDir
		}

		changes = append(changes, FileChange{
			Filepath:   fp,
			ChangeType: ChangeTypeInit,
		})

		return nil
	})
	if err != nil {
		return err
	}
	p.config.HandleCommit(Commit{
		Changes: changes,
		Sha:     commit.Hash.String(),
		When:    commit.Author.When.UTC(),
	})
	return nil
}

func (p *poller) setup() (*time.Ticker, error) {
	repo, err := p.git.Clone(p.config.Git.Remote, p.config.Git.Branch, p.config.Git.CloneDirectory)
	if err != nil {
		return nil, err
	}

	p.repo = repo

	err = p.onStart()
	if err != nil {
		return nil, err
	}

	return time.NewTicker(p.config.Interval), nil
}

func (p *poller) loop(ticker *time.Ticker) {
	for {
		select {
		case <-ticker.C:
			changes, err := p.Poll()
			if err != nil {
				continue
			}
			for _, c := range changes {
				if p.config.HandleCommit != nil {
					p.config.HandleCommit(c)
				}
				p.c <- c
			}
		case <-p.closer:
			ticker.Stop()
			return
		}
	}
}
