package ssh

import (
	"fmt"
	"os"
	"sync"
	"time"

	"sdt/internal/config"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	config *config.SSHConfig
	mu     sync.Mutex
	conn   *ssh.Client
}

func NewClient(cfg *config.SSHConfig) *Client {
	return &Client{
		config: cfg,
	}
}

func (c *Client) Connect() error {
	var authMethods []ssh.AuthMethod

	if c.config.IdentityFile != "" {
		key, err := os.ReadFile(c.config.IdentityFile)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("no hay metodo de autenticacion configurado: configura identity_file en config.yaml")
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

	c.mu.Lock()
	old := c.conn
	c.conn = conn
	c.mu.Unlock()

	if old != nil {
		old.Close()
	}
	return nil
}

// WHY: acotado a 3s — un socket muerto puede no dar error hasta que el SO lo note.
func (c *Client) IsAlive() bool {
	conn := c.GetConn()
	if conn == nil {
		return false
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := conn.SendRequest("keepalive@sdt", true, nil)
		done <- err
	}()

	select {
	case err := <-done:
		return err == nil
	case <-time.After(3 * time.Second):
		return false
	}
}

func (c *Client) EnsureConnected() error {
	if c.IsAlive() {
		return nil
	}
	return c.Connect()
}

func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (c *Client) Run(command string) (string, error) {
	conn := c.GetConn()
	if conn == nil {
		return "", fmt.Errorf("no hay conexion SSH activa")
	}

	session, err := conn.NewSession()
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

func (c *Client) RunInDir(dir, command string) (string, error) {
	fullCmd := fmt.Sprintf("cd %s && %s", dir, command)
	return c.Run(fullCmd)
}

func (c *Client) IsConnected() bool {
	return c.GetConn() != nil
}

func (c *Client) GetHost() string {
	return c.config.Host
}

func (c *Client) GetConn() *ssh.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

func (c *Client) RunStream(command string, outputCh chan<- string, stopCh <-chan struct{}) error {
	conn := c.GetConn()
	if conn == nil {
		return fmt.Errorf("no hay conexion SSH activa")
	}

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("error creando sesion: %v", err)
	}

	fullCmd := command
	if c.config.InitCmd != "" {
		fullCmd = fmt.Sprintf("%s && %s", c.config.InitCmd, command)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return err
	}

	if err := session.Start(fullCmd); err != nil {
		session.Close()
		return err
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-stopCh:
				session.Close()
				return
			default:
				n, err := stdout.Read(buf)
				if n > 0 {
					outputCh <- string(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-stopCh:
				return
			default:
				n, err := stderr.Read(buf)
				if n > 0 {
					outputCh <- string(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		<-stopCh
		session.Close()
	}()

	return nil
}
