package phone

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSocketArgs(t *testing.T) {
	ok := []map[string]any{
		{"target": "localabstract:chrome_devtools_remote", "http_path": "/json"},
		{"target": "tcp:9222", "send": "ping"},
		{"target": "localabstract:chrome_devtools_remote", "websocket_path": "/devtools/page/AB", "messages": []any{`{"id":1}`}},
	}
	for _, a := range ok {
		if _, _, m := parseSocketArgs(a); m != "" {
			t.Errorf("%v: %s", a, m)
		}
	}
	bad := []map[string]any{
		{"target": "localabstract:x y", "http_path": "/json"},
		{"target": "tcp:9222"},
		{"target": "tcp:9222", "send": "a", "http_path": "/json"},
		{"target": "file:/etc/passwd", "send": "a"},
		{"target": "tcp:9222", "http_path": "json"},
		{"target": "tcp:9222", "websocket_path": "/x"},
		{"target": "tcp:9222", "http_path": "/json", "http_method": "get /x"},
	}
	for _, a := range bad {
		if _, _, m := parseSocketArgs(a); m == "" {
			t.Errorf("accepted %v", a)
		}
	}
}

func serve(t *testing.T, h func(net.Conn)) net.Conn {
	a, b := net.Pipe()
	go func() { defer b.Close(); h(b) }()
	return a
}

func TestSocketHTTP(t *testing.T) {
	conn := serve(t, func(c net.Conn) {
		req, _ := http.ReadRequest(bufio.NewReader(c))
		fmt.Fprintf(c, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(req.URL.Path), req.URL.Path)
	})
	defer conn.Close()
	text, _, err := socketSession(conn, socketReq{HTTPPath: "/json/version", Method: "GET", Timeout: 3 * time.Second, MaxBytes: 1000})
	if err != nil || !strings.Contains(text, "HTTP 200") || !strings.Contains(text, "/json/version") {
		t.Fatalf("%q %v", text, err)
	}
}

func TestSocketRaw(t *testing.T) {
	conn := serve(t, func(c net.Conn) {
		buf := make([]byte, 16)
		n, _ := c.Read(buf)
		c.Write([]byte("echo:" + string(buf[:n])))
	})
	defer conn.Close()
	text, _, err := socketSession(conn, socketReq{Send: "hi", Wait: 300 * time.Millisecond, Timeout: 3 * time.Second, MaxBytes: 1000})
	if err != nil || text != "echo:hi" {
		t.Fatalf("%q %v", text, err)
	}
}

func TestSocketWebSocket(t *testing.T) {
	conn := serve(t, func(c net.Conn) {
		br := bufio.NewReader(c)
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		sum := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + wsGUID))
		fmt.Fprintf(c, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
		_, payload, err := wsRead(br)
		if err != nil {
			return
		}
		reply := `{"id":1,"result":"` + strings.ReplaceAll(string(payload), `"`, "'") + `"}`
		c.Write(append([]byte{0x81, byte(len(reply))}, reply...))
		io.Copy(io.Discard, c)
	})
	defer conn.Close()
	text, _, err := socketSession(conn, socketReq{WSPath: "/devtools/page/AB", Messages: []string{`{"id":1,"method":"Runtime.evaluate"}`}, Wait: time.Second, Timeout: 5 * time.Second, MaxBytes: 10000})
	if err != nil || !strings.Contains(text, `"id":1,\"result\"`) && !strings.Contains(text, "Runtime.evaluate") {
		t.Fatalf("%q %v", text, err)
	}
}
