package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogoAssets(t *testing.T) {
	for path, want := range map[string]string{
		"/logo.svg": "image/svg+xml", "/favicon.svg": "image/svg+xml",
		"/apple-touch-icon.png": "image/png", "/favicon.ico": "image/png",
	} {
		w := call("GET", path, "")
		if w.Code != 200 || w.Header().Get("content-type") != want || w.Body.Len() < 200 {
			t.Errorf("%s: %d %q %d bytes", path, w.Code, w.Header().Get("content-type"), w.Body.Len())
		}
	}
	if w := call("GET", "/apple-touch-icon.png", ""); !bytes.HasPrefix(w.Body.Bytes(), []byte("\x89PNG")) {
		t.Error("touch icon is not a PNG")
	}
	home := call("GET", "/", "").Body.String()
	for _, must := range []string{`rel="icon" href="/favicon.svg"`, `rel="apple-touch-icon"`, `src="https://raw.githubusercontent.com/theabbie/tailshell/master/plugin/assets/icon.png"`} {
		if !strings.Contains(home, must) {
			t.Errorf("home page is missing %s", must)
		}
	}
}
