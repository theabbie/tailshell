package adbx

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyRoundTripAgainstAdb(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "adbkey")
	if out, err := exec.Command("adb", "keygen", path).CombinedOutput(); err != nil {
		t.Skipf("local adb keygen unavailable: %v %s", err, out)
	}
	privWant, _ := os.ReadFile(path)
	pubWant, _ := os.ReadFile(path + ".pub")
	p, q, err := primesOf(privWant)
	if err != nil {
		t.Fatal(err)
	}
	key, err := keyFromPrimes(p, q)
	if err != nil {
		t.Fatal(err)
	}
	privGot, err := privPEM(key)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pem identical to adb's: %v", bytes.Equal(bytes.TrimSpace(privGot), bytes.TrimSpace(privWant)))
	pubGot := androidPubFile(key)
	wantB64 := strings.Fields(string(pubWant))[0]
	gotB64 := strings.Fields(string(pubGot))[0]
	if wantB64 != gotB64 {
		t.Fatalf("pub mismatch\nwant %s...\ngot  %s...", wantB64[:60], gotB64[:60])
	}
	t.Logf("derived .pub matches adb's (%d base64 chars); p+q = %d bytes", len(wantB64), len(p)+len(q))
}
