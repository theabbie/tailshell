package adbx

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
)

func fakeADB(t *testing.T, failService string) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				read := func() string {
					h := make([]byte, 4)
					if _, err := io.ReadFull(c, h); err != nil {
						return ""
					}
					var n int
					for _, ch := range string(h) {
						n = n*16 + strings.IndexRune("0123456789abcdef", ch)
					}
					b := make([]byte, n)
					io.ReadFull(c, b)
					return string(b)
				}
				if read() != "host:transport:127.0.0.1:1234" {
					c.Write([]byte("FAILxxxx"))
					return
				}
				c.Write([]byte("OKAY"))
				svc := read()
				if svc == failService {
					c.Write([]byte("FAIL0007refused"))
					return
				}
				c.Write([]byte("OKAY"))
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				c.Write([]byte(svc + "|" + string(buf[:n])))
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestStream(t *testing.T) {
	c := &Client{e: env{port: fakeADB(t, "localabstract:nope")}, serial: "127.0.0.1:1234"}
	conn, err := c.Stream(context.Background(), "localabstract:chrome_devtools_remote")
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("hello"))
	b, _ := io.ReadAll(conn)
	conn.Close()
	if string(b) != "localabstract:chrome_devtools_remote|hello" {
		t.Errorf("stream: %q", b)
	}
	if _, err := c.Stream(context.Background(), "localabstract:nope"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("refusal: %v", err)
	}
}
