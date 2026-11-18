package phone

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	targetRe = regexp.MustCompile(`^(localabstract:[A-Za-z0-9_.@-]{1,100}|localfilesystem:/[A-Za-z0-9_./-]{1,200}|tcp:[0-9]{1,5})$`)
	methodRe = regexp.MustCompile(`^[A-Z]{3,7}$`)
	pathRe   = regexp.MustCompile(`^/[^\s]{0,1000}$`)
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type socketReq struct {
	Send     string
	HTTPPath string
	Method   string
	Body     string
	WSPath   string
	Messages []string
	Wait     time.Duration
	Timeout  time.Duration
	MaxBytes int
}

func parseSocketArgs(a map[string]any) (target string, r socketReq, msg string) {
	gets := func(k string) string { s, _ := a[k].(string); return s }
	geti := func(k string, def, lo, hi int) int {
		v, ok := a[k].(float64)
		if !ok {
			return def
		}
		return max(lo, min(hi, int(v)))
	}
	target = gets("target")
	if !targetRe.MatchString(target) {
		return "", r, "target must be localabstract:NAME, localfilesystem:/PATH or tcp:PORT (a socket or port on the phone itself)"
	}
	r = socketReq{Send: gets("send"), HTTPPath: gets("http_path"), Method: strings.ToUpper(gets("http_method")), Body: gets("http_body"), WSPath: gets("websocket_path")}
	if r.Method == "" {
		r.Method = "GET"
	}
	for _, p := range []string{r.HTTPPath, r.WSPath} {
		if p != "" && !pathRe.MatchString(p) {
			return "", r, "http_path and websocket_path must start with / and contain no spaces"
		}
	}
	if !methodRe.MatchString(r.Method) {
		return "", r, "http_method must be an upper-case HTTP method such as GET or POST"
	}
	modes := 0
	for _, on := range []bool{r.Send != "", r.HTTPPath != "", r.WSPath != ""} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		return "", r, "set exactly one of send, http_path or websocket_path"
	}
	if raw, ok := a["messages"].([]any); ok {
		for _, m := range raw {
			switch v := m.(type) {
			case string:
				r.Messages = append(r.Messages, v)
			default:
				b, _ := json.Marshal(v)
				r.Messages = append(r.Messages, string(b))
			}
		}
	}
	if r.WSPath != "" && len(r.Messages) == 0 {
		return "", r, "websocket_path needs messages (strings or JSON objects to send as text frames)"
	}
	r.Wait = time.Duration(geti("wait_ms", 1500, 100, 20000)) * time.Millisecond
	r.Timeout = time.Duration(geti("timeout_ms", 10000, 1000, 30000)) * time.Millisecond
	r.MaxBytes = geti("max_bytes", 60000, 1000, 200000)
	return target, r, ""
}

// socketSession talks to an already opened stream: raw text, one HTTP request,
// or a WebSocket exchange. The caller closes the connection.
func socketSession(conn net.Conn, r socketReq) (string, any, error) {
	conn.SetDeadline(time.Now().Add(r.Timeout))
	switch {
	case r.WSPath != "":
		msgs, err := wsExchange(conn, r)
		out, _ := json.Marshal(msgs)
		return string(out), map[string]any{"messages": msgs}, err
	case r.HTTPPath != "":
		req := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\nContent-Length: %d\r\n\r\n%s", r.Method, r.HTTPPath, len(r.Body), r.Body)
		if _, err := io.WriteString(conn, req); err != nil {
			return "", nil, err
		}
		resp, err := http.ReadResponse(bufio.NewReader(io.LimitReader(conn, int64(r.MaxBytes)+65536)), nil)
		if err != nil {
			return "", nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(r.MaxBytes)))
		return fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, body), map[string]any{"status": resp.StatusCode, "body": string(body)}, nil
	default:
		if _, err := io.WriteString(conn, r.Send); err != nil {
			return "", nil, err
		}
		var out []byte
		buf := make([]byte, 4096)
		for len(out) < r.MaxBytes {
			idle := r.Wait
			if len(out) == 0 {
				idle = r.Timeout
			}
			conn.SetReadDeadline(time.Now().Add(idle))
			n, err := conn.Read(buf)
			out = append(out, buf[:n]...)
			if err != nil {
				break
			}
		}
		if len(out) > r.MaxBytes {
			out = out[:r.MaxBytes]
		}
		return string(out), map[string]any{"response": string(out)}, nil
	}
}

func wsExchange(conn net.Conn, r socketReq) ([]string, error) {
	key := make([]byte, 16)
	rand.Read(key)
	k := base64.StdEncoding.EncodeToString(key)
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", r.WSPath, k)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 101 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("websocket handshake refused: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	sum := sha1.Sum([]byte(k + wsGUID))
	if resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		return nil, errors.New("websocket handshake returned a bad accept key")
	}
	want := map[float64]bool{}
	for _, m := range r.Messages {
		if err := wsWrite(conn, 0x1, []byte(m)); err != nil {
			return nil, err
		}
		var probe struct {
			ID *float64 `json:"id"`
		}
		if json.Unmarshal([]byte(m), &probe) == nil && probe.ID != nil {
			want[*probe.ID] = true
		}
	}
	var got []string
	size := 0
	for size < r.MaxBytes {
		conn.SetReadDeadline(time.Now().Add(r.Wait))
		op, payload, err := wsRead(br)
		if err != nil {
			break
		}
		switch op {
		case 0x8:
			return got, nil
		case 0x9:
			wsWrite(conn, 0xA, payload)
		case 0x1:
			s := string(payload)
			got = append(got, s)
			size += len(s)
			var probe struct {
				ID *float64 `json:"id"`
			}
			if json.Unmarshal(payload, &probe) == nil && probe.ID != nil {
				delete(want, *probe.ID)
			}
			if len(want) == 0 && len(r.Messages) > 0 && anyID(r.Messages) {
				return got, nil
			}
		}
	}
	wsWrite(conn, 0x8, nil)
	return got, nil
}

func anyID(msgs []string) bool {
	for _, m := range msgs {
		var probe struct {
			ID *float64 `json:"id"`
		}
		if json.Unmarshal([]byte(m), &probe) == nil && probe.ID != nil {
			return true
		}
	}
	return false
}

func wsWrite(w io.Writer, op byte, payload []byte) error {
	hdr := []byte{0x80 | op}
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 65536:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	mask := make([]byte, 4)
	rand.Read(mask)
	hdr = append(hdr, mask...)
	body := make([]byte, n)
	for i := range payload {
		body[i] = payload[i] ^ mask[i%4]
	}
	_, err := w.Write(append(hdr, body...))
	return err
}

func wsRead(br *bufio.Reader) (byte, []byte, error) {
	var op byte
	var msg []byte
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(br, h); err != nil {
			return 0, nil, err
		}
		fin, o := h[0]&0x80 != 0, h[0]&0x0F
		n := uint64(h[1] & 0x7F)
		switch n {
		case 126:
			e := make([]byte, 2)
			if _, err := io.ReadFull(br, e); err != nil {
				return 0, nil, err
			}
			n = uint64(binary.BigEndian.Uint16(e))
		case 127:
			e := make([]byte, 8)
			if _, err := io.ReadFull(br, e); err != nil {
				return 0, nil, err
			}
			n = binary.BigEndian.Uint64(e)
		}
		if n > 4<<20 {
			return 0, nil, errors.New("websocket frame too large")
		}
		var mask []byte
		if h[1]&0x80 != 0 {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(br, mask); err != nil {
				return 0, nil, err
			}
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(br, p); err != nil {
			return 0, nil, err
		}
		for i := range p {
			if mask != nil {
				p[i] ^= mask[i%4]
			}
		}
		if o != 0 {
			op = o
		}
		msg = append(msg, p...)
		if fin || op >= 0x8 {
			return op, msg, nil
		}
	}
}
