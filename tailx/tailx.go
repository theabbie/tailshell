package tailx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tailscale.com/tsnet"
	"tailshell/adbx"
)

type entry struct {
	srv  *tsnet.Server
	refs int
}

var (
	mu    sync.Mutex
	nodes = map[[32]byte]*entry{}
)

type scopeKey struct{}

type scope struct {
	mu       sync.Mutex
	releases []func()
}

// Scope ties every node opened under the returned context to one request. The
// returned function must run before the request ends: it disconnects nodes that
// no other request in this instance is still using. A node only ever holds its
// connection to Tailscale while a request is in flight, so an instance that
// the platform freezes between requests never wakes up with a live connection
// that fights another instance for the same node key.
func Scope(ctx context.Context) (context.Context, func()) {
	s := &scope{}
	return context.WithValue(ctx, scopeKey{}, s), func() {
		s.mu.Lock()
		rs := s.releases
		s.releases = nil
		s.mu.Unlock()
		for _, r := range rs {
			r()
		}
	}
}

func hold(ctx context.Context, key [32]byte, e *entry) {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return
	}
	release := func() {
		mu.Lock()
		defer mu.Unlock()
		if e.refs--; e.refs <= 0 {
			e.srv.Close()
			if nodes[key] == e {
				delete(nodes, key)
			}
		}
	}
	s.mu.Lock()
	s.releases = append(s.releases, release)
	s.mu.Unlock()
}

type Device struct {
	Name    string   `json:"name"`
	DNSName string   `json:"dnsName"`
	OS      string   `json:"os"`
	IPs     []string `json:"ips"`
	Online  bool     `json:"online"`
}

func newServer(dir, authKey string) *tsnet.Server {
	return &tsnet.Server{
		Hostname: "tailshell",
		Dir:      dir,
		AuthKey:  authKey,
		// Not ephemeral: an ephemeral node is deleted once it goes offline, which
		// would invalidate the saved identity.
		Ephemeral: false,
		Logf:      func(string, ...any) {},
		UserLogf:  func(string, ...any) {},
	}
}

func Init(ctx context.Context, authKey string) ([]byte, error) {
	authKey = strings.TrimSpace(authKey)
	if !strings.HasPrefix(authKey, "tskey-") || len(authKey) > 300 {
		return nil, errors.New("that does not look like a Tailscale auth key (tskey-auth-...)")
	}
	rnd := make([]byte, 6)
	rand.Read(rnd)
	dir := filepath.Join(os.TempDir(), "ts-init-"+hex.EncodeToString(rnd))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	srv := newServer(dir, authKey)
	upCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if _, err := srv.Up(upCtx); err != nil {
		srv.Close()
		os.RemoveAll(dir)
		return nil, errors.New("could not join the tailnet with that key (expired, revoked, or wrong?): " + err.Error())
	}
	var state []byte
	var err error
	for i := 0; i < 30; i++ {
		if state, err = os.ReadFile(filepath.Join(dir, "tailscaled.state")); err == nil && len(state) > 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(state) <= 2 {
		srv.Close()
		return nil, errors.New("joined the tailnet but could not read the node state")
	}
	key := sha256.Sum256(state)
	mu.Lock()
	e := &entry{srv: srv, refs: 1}
	nodes[key] = e
	mu.Unlock()
	hold(ctx, key, e)
	return state, nil
}

func jitter(base time.Duration) time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(base)))
	if err != nil {
		return base
	}
	return base + time.Duration(n.Int64())
}

// Open connects the node saved in state, or reuses the one an in-flight request
// in this instance already connected. The first attempt gets a short timeout and
// one retry follows after a random pause, so a brief overlap with another holder
// of the same node key can pass.
func Open(ctx context.Context, state []byte) (*tsnet.Server, error) {
	if len(state) <= 2 {
		return nil, errors.New("no Tailscale state: pair again from the home page")
	}
	key := sha256.Sum256(state)
	mu.Lock()
	defer mu.Unlock()
	if e, ok := nodes[key]; ok {
		e.refs++
		hold(ctx, key, e)
		return e.srv, nil
	}
	dir := filepath.Join(os.TempDir(), "ts-"+hex.EncodeToString(key[:8]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	var lastErr error
	for attempt, timeout := range []time.Duration{20 * time.Second, 18 * time.Second} {
		if attempt > 0 {
			select {
			case <-time.After(jitter(1500 * time.Millisecond)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), state, 0o600); err != nil {
			return nil, err
		}
		srv := newServer(dir, "")
		upCtx, cancel := context.WithTimeout(ctx, timeout)
		_, err := srv.Up(upCtx)
		cancel()
		if err == nil {
			waitForPeers(ctx, srv)
			e := &entry{srv: srv, refs: 1}
			nodes[key] = e
			hold(ctx, key, e)
			return srv, nil
		}
		srv.Close()
		lastErr = err
	}
	return nil, errors.New("could not rejoin the tailnet from the saved state (machine deleted, key expired, or the node key is in use elsewhere at this moment; try again or create a new URL): " + lastErr.Error())
}

// waitForPeers blocks until the node has received its peer list. Up returns as
// soon as the node is running, which on a rejoin can be before the peers and
// MagicDNS names are known; dialing a peer by name in that window fails.
func waitForPeers(ctx context.Context, srv *tsnet.Server) {
	lc, err := srv.LocalClient()
	if err != nil {
		return
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := lc.Status(ctx); err == nil && len(st.Peer) > 0 {
			return
		}
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return
		}
	}
}

func Resolver(ctx context.Context, state []byte) (adbx.DialFunc, error) {
	srv, err := Open(ctx, state)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return srv.Dial(ctx, network, addr)
	}, nil
}

func Devices(ctx context.Context, state []byte) ([]Device, error) {
	srv, err := Open(ctx, state)
	if err != nil {
		return nil, err
	}
	lc, err := srv.LocalClient()
	if err != nil {
		return nil, err
	}
	st, err := lc.Status(ctx)
	if err != nil {
		return nil, err
	}
	var out []Device
	for _, p := range st.Peer {
		d := Device{Name: p.HostName, DNSName: strings.TrimSuffix(p.DNSName, "."), OS: p.OS, Online: p.Online}
		for _, ip := range p.TailscaleIPs {
			d.IPs = append(d.IPs, ip.String())
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func AllowedHost(h string) bool {
	h = strings.ToLower(h)
	if ip := net.ParseIP(h); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4[0] == 100 && v4[1]&0xC0 == 64
		}
		return strings.HasPrefix(ip.String(), "fd7a:115c:a1e0:")
	}
	return strings.HasSuffix(h, ".ts.net") || (h != "" && !strings.Contains(h, "."))
}
