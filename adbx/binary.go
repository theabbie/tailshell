package adbx

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func binDir() string { return filepath.Join(os.TempDir(), "tailshell-adb") }

func toolsURL() (string, error) {
	switch runtime.GOOS {
	case "linux", "darwin":
		return "https://dl.google.com/android/repository/platform-tools-latest-" + runtime.GOOS + ".zip", nil
	}
	return "", fmt.Errorf("no adb download for %s: install adb and put it on PATH", runtime.GOOS)
}

var (
	setupMu sync.Mutex
	adbBin  string
)

func ensureBinary(ctx context.Context) (string, error) {
	setupMu.Lock()
	defer setupMu.Unlock()
	if adbBin != "" {
		return adbBin, nil
	}
	if p, err := exec.LookPath("adb"); err == nil {
		adbBin = p
		return p, nil
	}
	dst := filepath.Join(binDir(), "adb")
	if _, err := os.Stat(dst); err == nil {
		adbBin = dst
		return dst, nil
	}
	dctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	url, err := toolsURL()
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(dctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download platform-tools: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download platform-tools: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir(), 0o755); err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != "platform-tools/adb" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return "", err
		}
		adbBin = dst
		return dst, nil
	}
	return "", errors.New("adb not found in platform-tools archive")
}

type env struct {
	home string
	port int
}

func envFor(s *session) env {
	h := sha256.Sum256(s.Priv)
	return env{
		home: filepath.Join(os.TempDir(), "tailshell-adb-"+hex.EncodeToString(h[:8])),
		port: 20000 + int(uint16(h[8])<<8|uint16(h[9]))%20000,
	}
}

func (e env) vars() []string {
	return append(os.Environ(), "HOME="+e.home, fmt.Sprintf("ANDROID_ADB_SERVER_PORT=%d", e.port))
}

func run(ctx context.Context, e env, timeout time.Duration, args ...string) ([]byte, error) {
	bin, err := ensureBinary(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(e.home, ".android"), 0o700); err != nil {
		return nil, err
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// stdout goes to a file, not a pipe: start-server forks a daemon that keeps the
	// pipe open, so reading it would block forever.
	tmp, err := os.CreateTemp("", "adbout-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	cmd := exec.CommandContext(rctx, bin, args...)
	cmd.Env = e.vars()
	cmd.Stdout, cmd.Stderr = tmp, tmp
	runErr := cmd.Run()
	out, _ := os.ReadFile(tmp.Name())
	if len(out) > 4<<20 {
		out = out[:4<<20]
	}
	if runErr != nil {
		name := strings.Join(args[:min(len(args), 2)], " ")
		if rctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("adb %s timed out after %s", name, timeout)
		}
		return out, fmt.Errorf("adb %s: %w", name, runErr)
	}
	return out, nil
}

func newKeyPair(ctx context.Context) (priv, pub []byte, err error) {
	dir, err := os.MkdirTemp("", "adbkey-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "adbkey")
	if out, err := run(ctx, env{home: dir, port: 5037}, 20*time.Second, "keygen", path); err != nil {
		return nil, nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	if priv, err = os.ReadFile(path); err != nil {
		return nil, nil, err
	}
	pub, err = os.ReadFile(path + ".pub")
	return priv, pub, err
}

func startServer(ctx context.Context, s *session) (env, error) {
	e := envFor(s)
	keyDir := filepath.Join(e.home, ".android")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return e, err
	}
	if err := os.WriteFile(filepath.Join(keyDir, "adbkey"), s.Priv, 0o600); err != nil {
		return e, err
	}
	if err := os.WriteFile(filepath.Join(keyDir, "adbkey.pub"), s.Pub, 0o644); err != nil {
		return e, err
	}
	out, err := run(ctx, e, 30*time.Second, "start-server")
	if err != nil {
		return e, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return e, nil
}
