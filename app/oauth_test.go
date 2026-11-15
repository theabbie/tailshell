package app

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func call(method, path, body string, headers ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	Handler(w, r)
	return w
}

func TestOAuthRoutes(t *testing.T) {
	t.Setenv("SEAL_SECRET", strings.Repeat("s", 32))

	w := call("POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if w.Code != 401 || !strings.Contains(w.Header().Get("WWW-Authenticate"), `resource_metadata="http`) || !strings.Contains(w.Header().Get("WWW-Authenticate"), "/.well-known/oauth-protected-resource") {
		t.Errorf("no bearer: %d %q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
	w = call("POST", "/mcp/", `{}`, "Authorization", "Bearer garbage")
	if w.Code != 401 || !strings.Contains(w.Header().Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Errorf("bad bearer: %d %q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
	if w = call("OPTIONS", "/mcp", ""); w.Code != 204 {
		t.Errorf("preflight: %d", w.Code)
	}
	if w = call("POST", "/mcp/garbage", `{}`); w.Code != 401 || w.Header().Get("WWW-Authenticate") != "" {
		t.Errorf("path-token mode must keep working unchanged: %d", w.Code)
	}

	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server", "/.well-known/oauth-authorization-server/mcp", "/.well-known/openid-configuration"} {
		if w = call("GET", path, ""); w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Errorf("%s: %d", path, w.Code)
		}
	}

	w = call("POST", "/oauth/register", `{"redirect_uris":["https://chatgpt.com/connector_platform_oauth_redirect"],"client_name":"ChatGPT"}`)
	var reg map[string]any
	json.Unmarshal(w.Body.Bytes(), &reg)
	id, _ := reg["client_id"].(string)
	if w.Code != 201 || id == "" {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	q := url.Values{"response_type": {"code"}, "client_id": {id}, "redirect_uri": {"https://chatgpt.com/connector_platform_oauth_redirect"}, "state": {"s1"}, "code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"S256"}}
	w = call("GET", "/oauth/authorize?"+q.Encode(), "")
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `"host":"chatgpt.com"`) || strings.Contains(body, "/*TS_OAUTH*/null") || w.Header().Get("x-frame-options") != "DENY" {
		t.Errorf("authorize page: %d", w.Code)
	}
	q.Set("redirect_uri", "https://evil.example/cb")
	if w = call("GET", "/oauth/authorize?"+q.Encode(), ""); w.Code != 400 {
		t.Errorf("unregistered redirect: %d", w.Code)
	}
	if w = call("GET", "/", ""); !strings.Contains(w.Body.String(), "/*TS_OAUTH*/null") {
		t.Error("the plain home page must not be in OAuth mode")
	}

	pair := `{"state":"AAAA","host":"100.64.0.1","pairPort":1,"code":"1","connectPort":2,"oauth":"garbage"}`
	if w = call("POST", "/api/pair", pair); w.Code != 400 || !strings.Contains(w.Body.String(), "authorization request") {
		t.Errorf("bad oauth context must fail before pairing: %d %s", w.Code, w.Body.String())
	}

	if w = call("POST", "/api/authorize", `{"oauth":"garbage","token":"x"}`); w.Code != 400 {
		t.Errorf("bad oauth context: %d", w.Code)
	}
	if w = call("POST", "/api/authorize", `{"oauth":"x"}`); w.Code != 400 {
		t.Errorf("missing token: %d", w.Code)
	}
}

func TestTokenFrom(t *testing.T) {
	for in, want := range map[string]string{
		"abc":                                "abc",
		"  abc  ":                            "abc",
		"Bearer abc":                         "abc",
		"https://x.example/mcp/abc":          "abc",
		"https://x.example/mcp/abc/":         "abc",
		"https://x.example/h/abc/screenshot": "abc",
		"https://x.example/h/abc":            "abc",
	} {
		if got := tokenFrom(in); got != want {
			t.Errorf("tokenFrom(%q) = %q, want %q", in, got, want)
		}
	}
}
