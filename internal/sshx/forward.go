package sshx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/northlighthq/boreal/internal/vault"
)

const (
	socksVersion  = 5
	socksConnect  = 1
	socksIPv4     = 1
	socksDomain   = 3
	socksIPv6     = 4
	socksNoAuth   = 0
	defaultBindIP = "127.0.0.1"
)

type Tunnel struct {
	Rule     vault.Forward
	listener net.Listener
	wg       sync.WaitGroup
	closed   chan struct{}
}

func (t *Tunnel) Close() error {
	select {
	case <-t.closed:
		return nil
	default:
	}

	close(t.closed)
	err := t.listener.Close()
	t.wg.Wait()
	return err
}

func (t *Tunnel) Done() <-chan struct{} {
	return t.closed
}

func StartForward(c *Client, r vault.Forward) (*Tunnel, error) {
	bind := r.BindAddr

	if bind == "" {
		bind = defaultBindIP
	}

	bindAddr := net.JoinHostPort(bind, strconv.Itoa(r.BindPort))
	dest := net.JoinHostPort(r.DestHost, strconv.Itoa(r.DestPort))

	var l net.Listener
	var err error
	var dial func() (net.Conn, error)

	switch r.Kind {
	case vault.ForwardLocal:
		l, err = net.Listen("tcp", bindAddr)
		dial = func() (net.Conn, error) { return c.Dial("tcp", dest) }
	case vault.ForwardRemote:
		l, err = c.Listen("tcp", bindAddr)
		dial = func() (net.Conn, error) { return net.Dial("tcp", dest) }
	case vault.ForwardDynamic:
		l, err = net.Listen("tcp", bindAddr)
	default:
		return nil, fmt.Errorf("unknown forwarding type: %q", r.Kind)
	}

	if err != nil {
		return nil, err
	}

	t := &Tunnel{Rule: r, listener: l, closed: make(chan struct{})}
	go t.accept(c, dial)
	return t, nil
}

func (t *Tunnel) accept(c *Client, dial func() (net.Conn, error)) {
	for {
		in, err := t.listener.Accept()

		if err != nil {
			select {
			case <-t.closed:
			default:
				close(t.closed)
			}

			return
		}

		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			defer in.Close()

			var out net.Conn
			var err error

			if t.Rule.Kind == vault.ForwardDynamic {
				out, err = socks5(in, c)
			} else {
				out, err = dial()
			}

			if err != nil {
				return
			}

			defer out.Close()
			pipe(in, out)
		}()
	}
}

func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		io.Copy(dst, src)
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
}

func socks5(conn net.Conn, c *Client) (net.Conn, error) {
	hdr := make([]byte, 2)

	if _, err := io.ReadFull(conn, hdr); err != nil {
		return nil, err
	}

	if hdr[0] != socksVersion {
		return nil, errors.New("socks: unsupported version")
	}

	methods := make([]byte, hdr[1])

	if _, err := io.ReadFull(conn, methods); err != nil {
		return nil, err
	}

	conn.Write([]byte{socksVersion, socksNoAuth})

	req := make([]byte, 4)

	if _, err := io.ReadFull(conn, req); err != nil {
		return nil, err
	}

	if req[1] != socksConnect {
		conn.Write([]byte{socksVersion, 7, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
		return nil, errors.New("socks: unsupported command")
	}

	var host string

	switch req[3] {
	case socksIPv4:
		ip := make([]byte, 4)

		if _, err := io.ReadFull(conn, ip); err != nil {
			return nil, err
		}

		host = net.IP(ip).String()
	case socksIPv6:
		ip := make([]byte, 16)

		if _, err := io.ReadFull(conn, ip); err != nil {
			return nil, err
		}

		host = net.IP(ip).String()
	case socksDomain:
		n := make([]byte, 1)

		if _, err := io.ReadFull(conn, n); err != nil {
			return nil, err
		}

		name := make([]byte, n[0])

		if _, err := io.ReadFull(conn, name); err != nil {
			return nil, err
		}

		host = string(name)
	default:
		return nil, errors.New("socks: unsupported address type")
	}

	pb := make([]byte, 2)

	if _, err := io.ReadFull(conn, pb); err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(pb))))
	out, err := c.Dial("tcp", addr)

	if err != nil {
		conn.Write([]byte{socksVersion, 5, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
		return nil, err
	}

	conn.Write([]byte{socksVersion, 0, 0, socksIPv4, 0, 0, 0, 0, 0, 0})
	return out, nil
}

func Describe(r vault.Forward) string {
	bind := r.BindAddr

	if bind == "" {
		bind = defaultBindIP
	}

	switch r.Kind {
	case vault.ForwardLocal:
		return fmt.Sprintf("%s:%d → %s:%d", bind, r.BindPort, r.DestHost, r.DestPort)
	case vault.ForwardRemote:
		return fmt.Sprintf("remote %s:%d → %s:%d", bind, r.BindPort, r.DestHost, r.DestPort)
	default:
		return fmt.Sprintf("SOCKS5 on %s:%d", bind, r.BindPort)
	}
}
