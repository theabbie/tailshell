// Package oauthx is a stateless OAuth 2.1 authorization server for the MCP
// endpoint at /mcp. It stores nothing: the client id, the authorization request
// and the authorization code are all sealed blobs (see adbx.SealBytes), and the
// access token it issues is the same sealed session token the /mcp/<token> URL
// carries.
package oauthx

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"tailshell/adbx"
)

const (
	clientLabel  = "tailshell-oauth-client|"
	requestLabel = "tailshell-oauth-request|"
	codeLabel    = "tailshell-oauth-code|"
	requestTTL   = 15 * time.Minute
	codeTTL      = 5 * time.Minute
	// The token never expires on its own; it stops working only when the
	// tailshell machine or the adb pairing is removed, or SEAL_SECRET changes.
	// A far-future lifetime avoids clients that treat a missing expires_in as
	// "expires soon" and then cannot refresh.
	accessLifetime = 315360000
)

var verifierRe = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)

type client struct {
	Name      string   `json:"n"`
	Redirects []string `json:"r"`
}

type Request struct {
	Client    string `json:"c"`
	Name      string `json:"n"`
	Redirect  string `json:"u"`
	State     string `json:"s"`
	Challenge string `json:"h"`
	Exp       int64  `json:"e"`
}

type code struct {
	Token     string `json:"t"`
	Client    string `json:"c"`
	Redirect  string `json:"u"`
	Challenge string `json:"h"`
	Exp       int64  `json:"e"`
}

func cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "content-type, authorization, mcp-protocol-version")
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	cors(w)
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("pragma", "no-cache")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func oauthErr(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func preflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	cors(w)
	w.WriteHeader(http.StatusNoContent)
	return true
}

func MetadataURL(origin string) string {
	return origin + "/.well-known/oauth-protected-resource/mcp"
}

func ProtectedResource(w http.ResponseWriter, r *http.Request, origin string) {
	if preflight(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 origin + "/mcp",
		"authorization_servers":    []string{origin},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "tailshell",
	})
}

func AuthServer(w http.ResponseWriter, r *http.Request, origin string) {
	if preflight(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                origin,
		"authorization_endpoint":                origin + "/oauth/authorize",
		"token_endpoint":                        origin + "/oauth/token",
		"registration_endpoint":                 origin + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"client_id_metadata_document_supported": true,
	})
}

// validRedirect follows the MCP spec: https, or http to a loopback host, with no
// fragment or credentials.
func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 512 || u.Fragment != "" || u.User != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

func seal(label string, v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return adbx.SealBytes(label, b)
}

func open(label, token string, v any) error {
	b, err := adbx.OpenBytes(label, token)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Register implements dynamic client registration (RFC 7591) without storage:
// the client id is the sealed set of redirect URIs.
func Register(w http.ResponseWriter, r *http.Request) {
	if preflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		oauthErr(w, http.StatusMethodNotAllowed, "invalid_request", "use POST")
		return
	}
	var in struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil || len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > 5 {
		oauthErr(w, http.StatusBadRequest, "invalid_client_metadata", "redirect_uris (1 to 5) is required")
		return
	}
	for _, u := range in.RedirectURIs {
		if !validRedirect(u) {
			oauthErr(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect URIs must use https (or http on localhost) without a fragment")
			return
		}
	}
	name := strings.TrimSpace(in.ClientName)
	if len(name) > 80 {
		name = name[:80]
	}
	id, err := seal(clientLabel, client{Name: name, Redirects: in.RedirectURIs})
	if err != nil {
		oauthErr(w, http.StatusServiceUnavailable, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  id,
		"client_id_issued_at":        time.Now().Unix(),
		"client_name":                name,
		"redirect_uris":              in.RedirectURIs,
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

// ParseAuthorize validates an authorization request. Any problem is reported to
// the user on an error page and never redirected, because the redirect target is
// only trusted once the client and its registered redirect URI check out.
func ParseAuthorize(r *http.Request) (*Request, error) {
	q := r.URL.Query()
	if q.Get("response_type") != "code" {
		return nil, errors.New("response_type must be code")
	}
	var c client
	if id := q.Get("client_id"); isCIMD(id) {
		var err error
		if c, err = fetchCIMD(r.Context(), id); err != nil {
			return nil, err
		}
	} else if err := open(clientLabel, id, &c); err != nil {
		return nil, errors.New("unknown client_id")
	}
	redirect := q.Get("redirect_uri")
	ok := false
	for _, u := range c.Redirects {
		if u == redirect {
			ok = true
		}
	}
	if !ok {
		return nil, errors.New("redirect_uri does not match the registered client")
	}
	if q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) < 43 || len(q.Get("code_challenge")) > 128 {
		return nil, errors.New("PKCE with code_challenge_method=S256 is required")
	}
	return &Request{
		Client: q.Get("client_id"), Name: c.Name, Redirect: redirect,
		State: q.Get("state"), Challenge: q.Get("code_challenge"),
		Exp: time.Now().Add(requestTTL).Unix(),
	}, nil
}

func (q *Request) Seal() (string, error) { return seal(requestLabel, q) }

func OpenRequest(token string) (*Request, error) {
	var q Request
	if err := open(requestLabel, token, &q); err != nil || time.Now().Unix() > q.Exp {
		return nil, errors.New("this authorization request is invalid or has expired; start again from the app")
	}
	return &q, nil
}

func (q *Request) redirectWith(params url.Values) string {
	u, _ := url.Parse(q.Redirect)
	v := u.Query()
	for k, vals := range params {
		v[k] = vals
	}
	if q.State != "" {
		v.Set("state", q.State)
	}
	u.RawQuery = v.Encode()
	return u.String()
}

// Deny is where the user goes when they cancel.
func (q *Request) Deny() string { return q.redirectWith(url.Values{"error": {"access_denied"}}) }

// Complete turns a freshly paired session token into the redirect that hands an
// authorization code back to the client.
func (q *Request) Complete(sessionToken string) (string, error) {
	c, err := seal(codeLabel, code{
		Token: sessionToken, Client: q.Client, Redirect: q.Redirect,
		Challenge: q.Challenge, Exp: time.Now().Add(codeTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	return q.redirectWith(url.Values{"code": {c}}), nil
}

// Token implements the token endpoint for public clients with PKCE. No refresh
// token is issued: when the session is gone for good, the client is sent back
// through authorization.
func Token(w http.ResponseWriter, r *http.Request) {
	if preflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		oauthErr(w, http.StatusMethodNotAllowed, "invalid_request", "use POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthErr(w, http.StatusBadRequest, "invalid_request", "expected a form-encoded body")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
	case "refresh_token":
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "no refresh tokens are issued; authorize again")
		return
	default:
		oauthErr(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}
	var c code
	if err := open(codeLabel, r.PostForm.Get("code"), &c); err != nil || time.Now().Unix() > c.Exp {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "the authorization code is invalid or has expired")
		return
	}
	if r.PostForm.Get("client_id") != c.Client || r.PostForm.Get("redirect_uri") != c.Redirect {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "client_id or redirect_uri does not match the authorization request")
		return
	}
	verifier := r.PostForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(verifier))
	if !verifierRe.MatchString(verifier) || subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(c.Challenge)) != 1 {
		oauthErr(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": c.Token,
		"token_type":   "Bearer",
		"expires_in":   accessLifetime,
	})
}
