package operator

import (
	"bytes"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"io/ioutil"
	"net"
	"os"
)

type CommandRes struct {
	StdOut []byte
	StdErr []byte
}

type CommandOperator interface {
	Execute(command string) (CommandRes, error)
	Upload(src io.Reader, remotePath string, mode string) error
	UploadFile(path string, remotePath string, mode string) error
}

type Callback func(CommandOperator) error

func ExecuteLocal(callback Callback) error {
	return callback(NewLocalOperator())
}

func ExecuteRemoteWithPassword(host string, port int, user string, password string, callback Callback) error {
	return executeRemote(host, port, user, ssh.Password(password), callback)
}

func ExecuteRemoteWithPrivateKey(host string, port int, user string, privateKey string, callback Callback) error {
	buffer, err := ioutil.ReadFile(expandPath(privateKey))
	if err != nil {
		return errors.Wrapf(err, "unable to parse private key: %s", privateKey)
	}

	var method ssh.AuthMethod
	key, err := ssh.ParsePrivateKey(buffer)

	if err != nil {
		if err.Error() != "ssh: this private key is passphrase protected" {
			return errors.Wrapf(err, "unable to parse private key: %s", privateKey)
		}

		sshAgent, closeAgent := privateKeyUsingSSHAgent(privateKey + ".pub")
		defer closeAgent()

		if sshAgent != nil {
			method = sshAgent
		} else {
			fmt.Printf("Enter passphrase for '%s': ", privateKey)
			STDIN := int(os.Stdin.Fd())
			bytePassword, _ := terminal.ReadPassword(STDIN)
			fmt.Println()

			key, err = ssh.ParsePrivateKeyWithPassphrase(buffer, bytePassword)
			if err != nil {
				return errors.Wrapf(err, "parse private key with passphrase failed: %s", privateKey)
			}
			method = ssh.PublicKeys(key)
		}
	} else {
		method = ssh.PublicKeys(key)
	}

	return executeRemote(host, port, user, method, callback)
}

func ExecuteRemote(host string, port int, user string, callback Callback) error {
	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))

	if err != nil {
		return errors.Wrapf(err, "unable to reach SSH Agent")
	}

	defer sshAgent.Close()

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%d", host, port)
	operator, err := NewSSHOperator(address, config)

	if err != nil {
		return errors.Wrapf(err, "unable to connect to %s over ssh", address)
	}

	defer operator.Close()

	return callback(operator)
}

func privateKeyUsingSSHAgent(publicKeyPath string) (ssh.AuthMethod, func() error) {
	if sshAgentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		sshAgent := agent.NewClient(sshAgentConn)

		keys, _ := sshAgent.List()
		if len(keys) == 0 {
			return nil, sshAgentConn.Close
		}

		pubkey, err := ioutil.ReadFile(expandPath(publicKeyPath))
		if err != nil {
			return nil, sshAgentConn.Close
		}

		authkey, _, _, _, err := ssh.ParseAuthorizedKey(pubkey)
		if err != nil {
			return nil, sshAgentConn.Close
		}
		parsedkey := authkey.Marshal()

		for _, key := range keys {
			if bytes.Equal(key.Blob, parsedkey) {
				return ssh.PublicKeysCallback(sshAgent.Signers), sshAgentConn.Close
			}
		}
	}
	return nil, func() error { return nil }
}

func executeRemote(host string, port int, user string, authMethod ssh.AuthMethod, callback Callback) error {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	address := fmt.Sprintf("%s:%d", host, port)
	operator, err := NewSSHOperator(address, config)

	if err != nil {
		return errors.Wrapf(err, "unable to connect to %s over ssh", address)
	}

	defer operator.Close()

	return callback(operator)
}

func expandPath(path string) string {
	res, _ := homedir.Expand(path)
	return res
}