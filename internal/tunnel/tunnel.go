package tunnel

import (
	"fmt"
	"io"
	"net"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

// WHY: interface y no *ssh.Client, para que una reconexión se tome sin recrear el túnel.
type connProvider interface {
	GetConn() *gossh.Client
}

type Tunnel struct {
	Name       string
	LocalPort  int
	RemoteHost string
	RemotePort int
	client     connProvider
	listener   net.Listener
	mu         sync.Mutex
	active     bool
	boundPort  int
}

func New(name string, localPort int, remoteHost string, remotePort int, client connProvider) *Tunnel {
	return &Tunnel{
		Name:       name,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		client:     client,
	}
}

func (t *Tunnel) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active {
		return nil
	}

	if t.LocalPort <= 0 {
		return fmt.Errorf("puerto local invalido: %d", t.LocalPort)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", t.LocalPort))
	if err != nil {
		return fmt.Errorf("puerto %d en uso o no disponible: %v", t.LocalPort, err)
	}

	t.listener = listener
	t.boundPort = listener.Addr().(*net.TCPAddr).Port
	t.active = true

	go t.accept()

	return nil
}

func (t *Tunnel) Port() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active {
		return t.boundPort
	}
	return t.LocalPort
}

func (t *Tunnel) SetLocalPort(port int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LocalPort = port
}

func (t *Tunnel) accept() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}
		go t.forward(conn)
	}
}

func (t *Tunnel) forward(local net.Conn) {
	defer local.Close()

	conn := t.client.GetConn()
	if conn == nil {
		return
	}

	remote, err := conn.Dial("tcp", fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort))
	if err != nil {
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 1)
	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()
	<-done
}

func (t *Tunnel) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listener != nil {
		t.listener.Close()
		t.listener = nil
	}
	t.active = false
}

func (t *Tunnel) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}
