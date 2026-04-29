package tunnel

import (
	"fmt"
	"io"
	"net"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

type Tunnel struct {
	Name       string
	LocalPort  int
	RemoteHost string
	RemotePort int
	sshConn    *gossh.Client
	listener   net.Listener
	mu         sync.Mutex
	active     bool
}

func New(name string, localPort int, remoteHost string, remotePort int, conn *gossh.Client) *Tunnel {
	return &Tunnel{
		Name:       name,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		sshConn:    conn,
	}
}

func (t *Tunnel) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active {
		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", t.LocalPort))
	if err != nil {
		return fmt.Errorf("puerto %d en uso o no disponible: %v", t.LocalPort, err)
	}

	t.listener = listener
	t.active = true

	go t.accept()

	return nil
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

	remote, err := t.sshConn.Dial("tcp", fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort))
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
