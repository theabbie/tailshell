package adbx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

type fwdEntry struct {
	addr string
	mu   sync.Mutex
	dial DialFunc
}

func (e *fwdEntry) set(d DialFunc) {
	e.mu.Lock()
	e.dial = d
	e.mu.Unlock()
}

func (e *fwdEntry) get() DialFunc {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dial
}

type forwarder struct {
	mu sync.Mutex
	m  map[string]*fwdEntry
}

var fwd = &forwarder{m: map[string]*fwdEntry{}}

func (f *forwarder) local(target string, dial DialFunc) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.m[target]; ok {
		e.set(dial)
		return e.addr, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	e := &fwdEntry{addr: ln.Addr().String(), dial: dial}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				rc, err := e.get()(ctx, "tcp", target)
				if err != nil {
					return
				}
				defer rc.Close()
				done := make(chan struct{}, 2)
				go func() { io.Copy(rc, c); done <- struct{}{} }()
				go func() { io.Copy(c, rc); done <- struct{}{} }()
				<-done
			}()
		}
	}()
	f.m[target] = e
	return e.addr, nil
}

func probe(ctx context.Context, dial DialFunc, target string) error {
	var err error
	for i := 0; i < 3; i++ {
		pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		c, derr := dial(pctx, "tcp", target)
		cancel()
		if err = derr; err == nil {
			c.Close()
			return nil
		}
		if i < 2 {
			time.Sleep(1500 * time.Millisecond)
		}
	}
	return err
}

func connect(ctx context.Context, e env, dial DialFunc, host string, port int) (string, string, error) {
	target := fmt.Sprintf("%s:%d", host, port)
	local, err := fwd.local(target, dial)
	if err != nil {
		return "", "", err
	}
	run(ctx, e, 10*time.Second, "disconnect", local)
	out, err := run(ctx, e, 15*time.Second, "connect", local)
	msg := strings.TrimSpace(string(out))
	if err != nil || !strings.Contains(msg, "connected to") {
		if perr := probe(ctx, dial, target); perr != nil {
			return "", "", fmt.Errorf("cannot reach %s over the tailnet: %v", target, perr)
		}
		out, err = run(ctx, e, 30*time.Second, "connect", local)
		msg = strings.TrimSpace(string(out))
		if err != nil || !strings.Contains(msg, "connected to") {
			return "", msg, fmt.Errorf("connect failed: %s", msg)
		}
	}
	state := ""
	for i := 0; i < 8; i++ {
		st, _ := run(ctx, e, 10*time.Second, "-s", local, "get-state")
		if state = strings.TrimSpace(string(st)); state == "device" {
			return local, msg, nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return "", msg, fmt.Errorf("connected, but the phone reports state %q (wireless debugging was toggled, a pairing dialog is open, or this pairing was removed)", state)
}

func Pair(ctx context.Context, dial DialFunc, netState []byte, host string, pairPort int, code string, connectPort int) (token, pairMsg, connectMsg string, err error) {
	if host == "" || pairPort <= 0 || code == "" {
		return "", "", "", errors.New("host, pairPort and code are required")
	}
	if _, err := sealKey(sealLabel); err != nil {
		return "", "", "", err
	}
	priv, pub, err := newKeyPair(ctx)
	if err != nil {
		return "", "", "", err
	}
	s := &session{Priv: priv, Pub: pub, Host: host, Port: connectPort, Net: netState}
	e, err := startServer(ctx, s)
	if err != nil {
		return "", "", "", err
	}
	local, err := fwd.local(fmt.Sprintf("%s:%d", host, pairPort), dial)
	if err != nil {
		return "", "", "", err
	}
	out, err := run(ctx, e, 40*time.Second, "pair", local, code)
	pairMsg = strings.TrimSpace(string(out))
	if err != nil || !strings.Contains(pairMsg, "Successfully paired") {
		return "", pairMsg, "", errors.New("pairing failed: " + pairMsg)
	}
	if token, err = seal(s); err != nil {
		return "", pairMsg, "", err
	}
	if connectPort > 0 {
		if _, msg, cerr := connect(ctx, e, dial, host, connectPort); cerr != nil {
			connectMsg = cerr.Error()
		} else {
			connectMsg = msg
		}
	}
	return token, pairMsg, connectMsg, nil
}

type DialResolver func(ctx context.Context, netState []byte) (DialFunc, error)

func Reconnect(ctx context.Context, resolve DialResolver, token, host string, port int) (string, string, error) {
	s, err := open(token)
	if err != nil {
		return "", "", err
	}
	dial, err := resolve(ctx, s.Net)
	if err != nil {
		return "", "", err
	}
	if host != "" {
		s.Host = host
	}
	s.Port = port
	e, err := startServer(ctx, s)
	if err != nil {
		return "", "", err
	}
	_, msg, err := connect(ctx, e, dial, s.Host, s.Port)
	if err != nil {
		return "", "", err
	}
	nt, err := seal(s)
	return nt, msg, err
}

// EnableTCP switches the phone's adbd to a fixed TCP port through the current
// connection and returns a session on that port, which keeps working when
// wireless debugging is off. The old session is untouched if any step fails.
func EnableTCP(ctx context.Context, resolve DialResolver, token string, port int) (string, error) {
	s, err := open(token)
	if err != nil {
		return "", err
	}
	dial, err := resolve(ctx, s.Net)
	if err != nil {
		return "", err
	}
	c, err := attach(ctx, s, dial)
	if err != nil {
		return "", err
	}
	out, err := c.Run(ctx, 20*time.Second, "tcpip", strconv.Itoa(port))
	if msg := strings.TrimSpace(string(out)); err != nil || !strings.Contains(msg, "TCP mode") {
		return "", fmt.Errorf("adb tcpip failed: %s", msg)
	}
	run(ctx, c.e, 10*time.Second, "disconnect", c.serial)
	s.Port = port
	for i := 0; ; i++ {
		if _, _, err = connect(ctx, c.e, dial, s.Host, port); err == nil {
			return seal(s)
		}
		if i == 3 {
			return "", fmt.Errorf("switched to TCP port %d but could not connect to it: %w", port, err)
		}
		time.Sleep(2 * time.Second)
	}
}

type Client struct {
	e      env
	serial string
	Host   string
	Port   int
}

func Attach(ctx context.Context, token string, resolve DialResolver) (*Client, error) {
	s, err := open(token)
	if err != nil {
		return nil, err
	}
	dial, err := resolve(ctx, s.Net)
	if err != nil {
		return nil, err
	}
	return attach(ctx, s, dial)
}

func attach(ctx context.Context, s *session, dial DialFunc) (*Client, error) {
	if s.Port <= 0 {
		return nil, errors.New("this session has no connect port; re-pair or update the port")
	}
	e, err := startServer(ctx, s)
	if err != nil {
		return nil, err
	}
	serial, _, err := connect(ctx, e, dial, s.Host, s.Port)
	if err != nil {
		return nil, fmt.Errorf("%w (wireless debugging off, or its port changed)", err)
	}
	return &Client{e: e, serial: serial, Host: s.Host, Port: s.Port}, nil
}

func (c *Client) Run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	return run(ctx, c.e, timeout, append([]string{"-s", c.serial}, args...)...)
}

func (c *Client) Shell(ctx context.Context, cmd string) (string, error) {
	out, err := c.Run(ctx, 45*time.Second, "shell", cmd)
	return string(out), err
}

func (c *Client) Screenshot(ctx context.Context) ([]byte, error) {
	out, err := c.Run(ctx, 30*time.Second, "exec-out", "screencap", "-p")
	if err != nil || len(out) < 8 {
		return nil, errors.New("screencap failed: " + strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Stream opens a raw byte stream to a service on the phone through the adb
// server, e.g. "localabstract:chrome_devtools_remote". It is what adb forward
// does behind a local port, without needing the port.
func (c *Client) Stream(ctx context.Context, service string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", c.e.port))
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	for _, req := range []string{"host:transport:" + c.serial, service} {
		if _, err := fmt.Fprintf(conn, "%04x%s", len(req), req); err != nil {
			conn.Close()
			return nil, err
		}
		status := make([]byte, 4)
		if _, err := io.ReadFull(conn, status); err != nil {
			conn.Close()
			return nil, err
		}
		if string(status) != "OKAY" {
			msg := "adb refused the request"
			if string(status) == "FAIL" {
				hdr := make([]byte, 4)
				if _, err := io.ReadFull(conn, hdr); err == nil {
					var n int
					fmt.Sscanf(string(hdr), "%x", &n)
					body := make([]byte, min(n, 512))
					io.ReadFull(conn, body)
					msg = string(body)
				}
			}
			conn.Close()
			return nil, errors.New(msg)
		}
	}
	return conn, nil
}
