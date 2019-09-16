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
	StartAsync() (chan GitChange, error)

	// Start polling your git repo blocking whatever thread it is run on. The poller will diff the remote against the
	// local clone directory at the specified interval and return all changes through the configured callback.
	Start() error

	// Stop all polling.
	Stop()

	// Diff the remote and the local and return all differences.
	Poll() ([]GitChange, error)
}

type GitAuthConfig struct {
	// The filepath to the SSH key. Required if the Username and Password are not set.
	SshKey string `validation:"required_without=Username Password"`

	// The username for the git repo. Required if the SshKey is not set or if the Password is set.
	Username string `validation:"required_without=SshKey,required_with=Password"`

	// The password for the git repo. Required if the SshKey is not set or if the Username is set.
	Password string `validation:"require_without=SshKey,required_with=Username"`
}

type HandleFunc func(change GitChange)

type FilterFunc func(change GitChange) bool

type PollConfig struct {
	Git GitConfig `validate:"required"`

	// Function that is called when a change is detected. If true is returned for the change, The function set for
	// HandleChange will trigger. If false is returned, HandleChange will not be called.
	Filter FilterFunc

	// Function that is called when a change occurs to a file in the git repository.
	HandleChange HandleFunc

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
	onChangeChan := make(chan GitChange, 1)

	poller := &poller{
		c:      onChangeChan,
		config: &config,
		closer: closer,
		git:    g,
	}

	return poller, nil
}

type poller struct {
	c      chan GitChange
	config *PollConfig
	closer chan bool
	git    gitService
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

func (p *poller) StartAsync() (chan GitChange, error) {
	ticker, err := p.setup()
	if err != nil {
		return nil, err
	}

	go p.loop(ticker)

	return p.c, nil
}

func (p *poller) Poll() ([]GitChange, error) {
	changes, err := p.git.DiffRemote(p.repo, p.config.Git.Branch)
	if err != nil {
		return nil, err
	}

	if len(changes) > 0 {
		if p.config.Filter != nil {
			filteredChanges := make([]GitChange, 0)
			for _, c := range changes {
				if p.config.Filter(c) {
					filteredChanges = append(filteredChanges, c)
				}
			}
			changes = filteredChanges
		}
		for i, c := range changes {
			changes[i].Filepath = path.Join(p.config.Git.CloneDirectory, c.Filepath)
		}
	}
	return changes, nil
}

func (p *poller) Stop() {
	p.closer <- true
}

func (p *poller) onStart() error {
	if p.config.HandleChange != nil {
		hash, err := p.git.HeadHash(p.repo)
		if err != nil {
			return err
		}
		return filepath.Walk(p.config.Git.CloneDirectory, func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			p.config.HandleChange(GitChange{
				Filepath:   path,
				Sha:        hash,
				ChangeType: ChangeTypeCreate,
			})
			return nil
		})
	}
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
				if p.config.HandleChange != nil {
					p.config.HandleChange(c)
				}
				p.c <- c
			}
		case <-p.closer:
			ticker.Stop()
			return
		}
	}
}
