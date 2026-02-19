package ssh

import (
	"fmt"
	"os"
	"time"

	"ssh-deploy-tui/internal/config"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	config *config.SSHConfig
	conn   *ssh.Client
}

func NewClient(cfg *config.SSHConfig) *Client {
	return &Client{
		config: cfg,
	}
}

func (c *Client) Connect() error {
	var authMethods []ssh.AuthMethod

	if c.config.KeyPath != "" {
		key, err := os.ReadFile(c.config.KeyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if c.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(c.config.Password))
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no hay metodo de autenticacion configurado (key_path o password)")
	}

	sshConfig := &ssh.ClientConfig{
		User:            c.config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("error conectando a %s: %v", addr, err)
	}

	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Run(command string) (string, error) {
	if c.conn == nil {
		return "", fmt.Errorf("no hay conexion SSH activa")
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("error creando sesion: %v", err)
	}
	defer session.Close()

	fullCmd := command
	if c.config.InitCmd != "" {
		fullCmd = fmt.Sprintf("%s && %s", c.config.InitCmd, command)
	}

	output, err := session.CombinedOutput(fullCmd)
	if err != nil {
		return string(output), fmt.Errorf("%s", string(output))
	}

	return string(output), nil
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) GetHost() string {
	return c.config.Host
}
