package oauthx

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func challenge() string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func register(t *testing.T, redirects ...string) string {
	body, _ := json.Marshal(map[string]any{"redirect_uris": redirects, "client_name": "ChatGPT"})
	w := httptest.NewRecorder()
	Register(w, httptest.NewRequest("POST", "/oauth/register", strings.NewReader(string(body))))
	if w.Code != 201 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return out["client_id"].(string)
}

func authorizeReq(clientID, redirect string) *http.Request {
	q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect}, "state": {"xyz"}, "code_challenge": {challenge()}, "code_challenge_method": {"S256"}}
	return httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
}

func tokenReq(form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	Token(w, req)
	return w
}

func TestFullFlow(t *testing.T) {
	t.Setenv("SEAL_SECRET", strings.Repeat("s", 32))
	redirect := "https://chatgpt.com/connector_platform_oauth_redirect"
	id := register(t, redirect)

	q, err := ParseAuthorize(authorizeReq(id, redirect))
	if err != nil {
		t.Fatal(err)
	}
	sealed, _ := q.Seal()
	back, err := OpenRequest(sealed)
	if err != nil || back.Redirect != redirect {
		t.Fatalf("request round trip: %v", err)
	}

	to, err := back.Complete("SESSION-TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(to)
	if u.Query().Get("state") != "xyz" || !strings.HasPrefix(to, redirect) {
		t.Fatalf("bad redirect: %s", to)
	}
	code := u.Query().Get("code")
	form := func() url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id}, "redirect_uri": {redirect}, "code_verifier": {verifier}}
	}

	w := tokenReq(form())
	var tok map[string]any
	json.Unmarshal(w.Body.Bytes(), &tok)
	if w.Code != 200 || tok["access_token"] != "SESSION-TOKEN" || tok["token_type"] != "Bearer" || tok["refresh_token"] != nil {
		t.Fatalf("token: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("cache-control") != "no-store" {
		t.Error("token response must not be cacheable")
	}

	bad := func(name string, mutate func(url.Values), want string) {
		f := form()
		mutate(f)
		w := tokenReq(f)
		var e map[string]string
		json.Unmarshal(w.Body.Bytes(), &e)
		if w.Code != 400 || e["error"] != want {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	bad("wrong verifier", func(f url.Values) { f.Set("code_verifier", strings.Repeat("a", 43)) }, "invalid_grant")
	bad("short verifier", func(f url.Values) { f.Set("code_verifier", "short") }, "invalid_grant")
	bad("other redirect", func(f url.Values) { f.Set("redirect_uri", "https://evil.example/cb") }, "invalid_grant")
	bad("other client", func(f url.Values) { f.Set("client_id", id+"x") }, "invalid_grant")
	bad("garbage code", func(f url.Values) { f.Set("code", "garbage") }, "invalid_grant")
	bad("refresh grant", func(f url.Values) { f.Set("grant_type", "refresh_token") }, "invalid_grant")
	bad("other grant", func(f url.Values) { f.Set("grant_type", "password") }, "unsupported_grant_type")

	expired, _ := seal(codeLabel, code2{"T", id, redirect, challenge(), time.Now().Add(-time.Minute).Unix()})
	f := form()
	f.Set("code", expired)
	if w := tokenReq(f); w.Code != 400 {
		t.Errorf("expired code accepted: %d", w.Code)
	}
}

type code2 = code

func TestAuthorizeRejections(t *testing.T) {
	t.Setenv("SEAL_SECRET", strings.Repeat("s", 32))
	redirect := "https://chatgpt.com/cb"
	id := register(t, redirect)
	if _, err := ParseAuthorize(authorizeReq(id, "https://evil.example/cb")); err == nil {
		t.Error("unregistered redirect accepted")
	}
	if _, err := ParseAuthorize(authorizeReq("garbage", redirect)); err == nil {
		t.Error("unknown client accepted")
	}
	r := authorizeReq(id, redirect)
	qs := r.URL.Query()
	qs.Set("code_challenge_method", "plain")
	r.URL.RawQuery = qs.Encode()
	if _, err := ParseAuthorize(r); err == nil {
		t.Error("non-S256 PKCE accepted")
	}
	qs.Del("code_challenge")
	qs.Set("code_challenge_method", "S256")
	r.URL.RawQuery = qs.Encode()
	if _, err := ParseAuthorize(r); err == nil {
		t.Error("missing challenge accepted")
	}
	expired, _ := seal(requestLabel, Request{Redirect: redirect, Exp: time.Now().Add(-time.Minute).Unix()})
	if _, err := OpenRequest(expired); err == nil {
		t.Error("expired authorization request accepted")
	}
	if _, err := OpenRequest("garbage"); err == nil {
		t.Error("garbage request accepted")
	}
}

func TestRegisterValidation(t *testing.T) {
	t.Setenv("SEAL_SECRET", strings.Repeat("s", 32))
	for _, bad := range []string{"http://evil.example/cb", "https://a.example/cb#frag", "javascript:alert(1)", "https://u:p@a.example/cb", "ftp://a.example/cb", "/relative"} {
		body, _ := json.Marshal(map[string]any{"redirect_uris": []string{bad}})
		w := httptest.NewRecorder()
		Register(w, httptest.NewRequest("POST", "/oauth/register", strings.NewReader(string(body))))
		if w.Code != 400 {
			t.Errorf("%q accepted: %d", bad, w.Code)
		}
	}
	for _, ok := range []string{"https://claude.ai/api/mcp/auth_callback", "http://localhost:6274/oauth/callback", "http://127.0.0.1:1234/cb"} {
		if register(t, ok) == "" {
			t.Errorf("%q rejected", ok)
		}
	}
	w := httptest.NewRecorder()
	Register(w, httptest.NewRequest("POST", "/oauth/register", strings.NewReader("{}")))
	if w.Code != 400 {
		t.Errorf("empty registration accepted: %d", w.Code)
	}
}

func TestMetadata(t *testing.T) {
	var as, pr map[string]any
	w := httptest.NewRecorder()
	AuthServer(w, httptest.NewRequest("GET", "/", nil), "https://h.example")
	json.Unmarshal(w.Body.Bytes(), &as)
	w = httptest.NewRecorder()
	ProtectedResource(w, httptest.NewRequest("GET", "/", nil), "https://h.example")
	json.Unmarshal(w.Body.Bytes(), &pr)
	if as["issuer"] != "https://h.example" || as["token_endpoint"] != "https://h.example/oauth/token" || as["registration_endpoint"] != "https://h.example/oauth/register" {
		t.Errorf("authorization server metadata: %v", as)
	}
	if pr["resource"] != "https://h.example/mcp" || pr["authorization_servers"].([]any)[0] != "https://h.example" {
		t.Errorf("protected resource metadata: %v", pr)
	}
}
