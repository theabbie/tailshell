package tailx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	used time.Time
}

const maxNodes = 4

var (
	mu    sync.Mutex
	nodes = map[[32]byte]*entry{}
)

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

func evictLocked() {
	for len(nodes) >= maxNodes {
		var oldest [32]byte
		var t time.Time
		first := true
		for k, e := range nodes {
			if first || e.used.Before(t) {
				oldest, t, first = k, e.used, false
			}
		}
		nodes[oldest].srv.Close()
		delete(nodes, oldest)
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
	mu.Lock()
	evictLocked()
	nodes[sha256.Sum256(state)] = &entry{srv: srv, used: time.Now()}
	mu.Unlock()
	return state, nil
}

func Open(ctx context.Context, state []byte) (*tsnet.Server, error) {
	if len(state) <= 2 {
		return nil, errors.New("no Tailscale state: pair again from the home page")
	}
	key := sha256.Sum256(state)
	mu.Lock()
	defer mu.Unlock()
	if e, ok := nodes[key]; ok {
		e.used = time.Now()
		return e.srv, nil
	}
	evictLocked()
	dir := filepath.Join(os.TempDir(), "ts-"+hex.EncodeToString(key[:8]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), state, 0o600); err != nil {
		return nil, err
	}
	srv := newServer(dir, "")
	upCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if _, err := srv.Up(upCtx); err != nil {
		srv.Close()
		return nil, errors.New("could not rejoin the tailnet from the saved state (machine deleted or key expired? create a new URL): " + err.Error())
	}
	nodes[key] = &entry{srv: srv, used: time.Now()}
	return srv, nil
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
