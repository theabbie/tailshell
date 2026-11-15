package adbx

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

type session struct {
	Priv []byte
	Pub  []byte
	Host string
	Port int
	Net  []byte
}

const (
	sealEnv         = "SEAL_SECRET"
	sealLabel       = "tailshell-session|"
	legacySealLabel = "abbienet-adb-session|"
)

func SealSecretConfigured() bool { return len(os.Getenv(sealEnv)) >= 16 }

func sealKey(label string) ([]byte, error) {
	t := os.Getenv(sealEnv)
	if len(t) < 16 {
		return nil, errors.New(sealEnv + " must be set to a random string of at least 16 characters")
	}
	k := sha256.Sum256([]byte(label + t))
	return k[:], nil
}

func newGCM(label string) (cipher.AEAD, error) {
	key, err := sealKey(label)
	if err != nil {
		return nil, err
	}
	blk, _ := aes.NewCipher(key)
	return cipher.NewGCM(blk)
}

// ------------------------------------------------------------- binary payload
//
// v3 payload (deflated after a 0x03 marker): TLV fields
//
//	1: p||q  (adb RSA primes, 256 bytes)
//	2: host  (string)
//	3: port  (uvarint)
//	4: tsnet state as repeated (uvarint klen, key, uvarint vlen, value)
//
// The tsnet state file is a JSON map of base64 values; the values are unwrapped
// to raw bytes first so the JSON/text inside them compresses.

func putField(b *bytes.Buffer, tag byte, data []byte) {
	b.WriteByte(tag)
	b.Write(binary.AppendUvarint(nil, uint64(len(data))))
	b.Write(data)
}

func packNet(state []byte) ([]byte, error) {
	var m map[string][]byte
	if err := json.Unmarshal(state, &m); err != nil {
		return nil, errors.New("tsnet state is not valid")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	for _, k := range keys {
		b.Write(binary.AppendUvarint(nil, uint64(len(k))))
		b.WriteString(k)
		b.Write(binary.AppendUvarint(nil, uint64(len(m[k]))))
		b.Write(m[k])
	}
	return b.Bytes(), nil
}

func unpackNet(data []byte) ([]byte, error) {
	m := map[string][]byte{}
	r := bytes.NewReader(data)
	read := func() ([]byte, error) {
		n, err := binary.ReadUvarint(r)
		if err != nil || n > uint64(r.Len()) {
			return nil, errors.New("corrupt state")
		}
		buf := make([]byte, n)
		_, err = io.ReadFull(r, buf)
		return buf, err
	}
	for r.Len() > 0 {
		k, err := read()
		if err != nil {
			return nil, err
		}
		v, err := read()
		if err != nil {
			return nil, err
		}
		m[string(k)] = v
	}
	return json.Marshal(m)
}

func encodeSession(s *session) ([]byte, error) {
	p, q, err := primesOf(s.Priv)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	putField(&b, 1, append(p, q...))
	putField(&b, 2, []byte(s.Host))
	putField(&b, 3, binary.AppendUvarint(nil, uint64(s.Port)))
	if len(s.Net) > 0 {
		net, err := packNet(s.Net)
		if err != nil {
			return nil, err
		}
		putField(&b, 4, net)
	}
	return b.Bytes(), nil
}

func decodeSession(data []byte) (*session, error) {
	s := &session{}
	r := bytes.NewReader(data)
	var pq []byte
	for r.Len() > 0 {
		tag, _ := r.ReadByte()
		n, err := binary.ReadUvarint(r)
		if err != nil || n > uint64(r.Len()) {
			return nil, errors.New("corrupt session")
		}
		f := make([]byte, n)
		io.ReadFull(r, f)
		switch tag {
		case 1:
			pq = f
		case 2:
			s.Host = string(f)
		case 3:
			v, _ := binary.Uvarint(f)
			s.Port = int(v)
		case 4:
			if s.Net, err = unpackNet(f); err != nil {
				return nil, err
			}
		}
	}
	if len(pq) != 256 {
		return nil, errors.New("corrupt session")
	}
	key, err := keyFromPrimes(pq[:128], pq[128:])
	if err != nil {
		return nil, err
	}
	if s.Priv, err = privPEM(key); err != nil {
		return nil, err
	}
	s.Pub = androidPubFile(key)
	return s, nil
}

type legacyWire struct {
	V    int    `json:"v,omitempty"`
	K    string `json:"k,omitempty"`
	P    string `json:"p,omitempty"`
	H    string `json:"h,omitempty"`
	N    int    `json:"n,omitempty"`
	Priv []byte `json:"priv,omitempty"`
	Pub  []byte `json:"pub,omitempty"`
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

func decodeLegacy(js []byte) (*session, error) {
	var w legacyWire
	if err := json.Unmarshal(js, &w); err != nil {
		return nil, errors.New("invalid session token")
	}
	if w.V == 2 {
		return &session{Priv: []byte(w.K), Pub: []byte(w.P), Host: w.H, Port: w.N}, nil
	}
	if len(w.Priv) == 0 {
		return nil, errors.New("invalid session token")
	}
	return &session{Priv: w.Priv, Pub: w.Pub, Host: w.Host, Port: w.Port}, nil
}

func deflate(b []byte) []byte {
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.BestCompression)
	fw.Write(b)
	fw.Close()
	return buf.Bytes()
}

func inflate(b []byte) ([]byte, error) {
	return io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(b)), 1<<20))
}

func seal(s *session) (string, error) {
	gcm, err := newGCM(sealLabel)
	if err != nil {
		return "", err
	}
	body, err := encodeSession(s)
	if err != nil {
		return "", err
	}
	plain := append([]byte{3}, deflate(body)...)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
}

func open(token string) (*session, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return nil, errors.New("invalid session token")
	}
	var plain []byte
	for _, label := range []string{sealLabel, legacySealLabel} {
		gcm, err := newGCM(label)
		if err != nil {
			return nil, err
		}
		if len(raw) < gcm.NonceSize() {
			continue
		}
		if p, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil); err == nil && len(p) > 0 {
			plain = p
			break
		}
	}
	if plain == nil {
		return nil, errors.New("invalid session token")
	}
	switch plain[0] {
	case 3:
		body, err := inflate(plain[1:])
		if err != nil {
			return nil, errors.New("invalid session token")
		}
		return decodeSession(body)
	case 1:
		js, err := inflate(plain[1:])
		if err != nil {
			return nil, errors.New("invalid session token")
		}
		return decodeLegacy(js)
	default:
		return decodeLegacy(plain)
	}
}

func CheckToken(token string) error {
	_, err := open(token)
	return err
}

func NetState(token string) ([]byte, error) {
	s, err := open(token)
	if err != nil {
		return nil, err
	}
	return s.Net, nil
}

// ErrReauth marks failures that retrying cannot fix: the saved identity is gone
// (the tailshell machine was removed or its key expired), so the caller must
// authorize again.
var ErrReauth = errors.New("authorization is no longer valid")

// SealBytes encrypts plain for one purpose. Different labels derive different
// keys, so a blob sealed for one purpose cannot be opened as another.
func SealBytes(label string, plain []byte) (string, error) {
	gcm, err := newGCM(label)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
}

func OpenBytes(label, token string) ([]byte, error) {
	gcm, err := newGCM(label)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) < gcm.NonceSize() {
		return nil, errors.New("invalid token")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	return plain, nil
}
