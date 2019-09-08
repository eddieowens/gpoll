package gpoll

import (
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func usernamePassword(username, password string) (transport.AuthMethod, error) {
	return &http.BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

func sshKeyFromFile(fp string) (transport.AuthMethod, error) {
	if strings.HasPrefix(fp, "~/") {
		home, _ := os.UserHomeDir()
		fp = path.Join(home, fp[2:])
	}
	key, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	return sshKey(key)
}

func sshKey(key []byte) (transport.AuthMethod, error) {
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}, nil
}

func toAuthMethod(config *GitAuthConfig) (transport.AuthMethod, error) {
	if config.SshKey != "" {
		return sshKeyFromFile(config.SshKey)
	} else {
		return usernamePassword(config.Username, config.Password)
	}
}
