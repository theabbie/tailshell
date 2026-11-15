package app

import (
	"strings"
	"testing"
)

func TestPublicPages(t *testing.T) {
	for path, must := range map[string]string{
		"/privacy": "Privacy policy",
		"/terms":   "Terms of use",
		"/support": "Support",
	} {
		w := call("GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), must) || !strings.Contains(w.Header().Get("content-type"), "text/html") {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
	if w := call("GET", "/", ""); !strings.Contains(w.Body.String(), `href="/privacy"`) || !strings.Contains(w.Body.String(), `href="/terms"`) {
		t.Error("home page footer is missing the legal links")
	}
}

func TestDomainChallenge(t *testing.T) {
	t.Setenv("OPENAI_APPS_CHALLENGE", "")
	if w := call("GET", "/.well-known/openai-apps-challenge", ""); w.Code != 404 {
		t.Errorf("without a token configured: %d", w.Code)
	}
	t.Setenv("OPENAI_APPS_CHALLENGE", "  abc123token\n")
	w := call("GET", "/.well-known/openai-apps-challenge", "")
	if w.Code != 200 || w.Body.String() != "abc123token" || !strings.HasPrefix(w.Header().Get("content-type"), "text/plain") {
		t.Errorf("challenge must be exactly the token as plain text: %d %q", w.Code, w.Body.String())
	}
}
