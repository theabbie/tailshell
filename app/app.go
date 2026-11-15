package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"tailshell/adbx"
	"tailshell/httpx"
	"tailshell/mcpx"
	"tailshell/oauthx"
	"tailshell/tailx"
	"tailshell/web"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func readBody(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

func decodeState(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) < 3 {
		return nil, errors.New("invalid node state")
	}
	return b, nil
}

func origin(r *http.Request) string {
	scheme := "https"
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	} else if r.TLS == nil && strings.HasPrefix(r.Host, "localhost") {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func Handler(w http.ResponseWriter, r *http.Request) {
	ctx, done := tailx.Scope(r.Context())
	defer done()
	r = r.WithContext(ctx)
	p := strings.TrimRight(r.URL.Path, "/")
	switch {
	case p == "" && r.Method == http.MethodGet:
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		w.Write([]byte(web.Page))

	case strings.HasPrefix(p, "/h/"):
		token, rest, _ := strings.Cut(strings.TrimPrefix(p, "/h/"), "/")
		httpx.Serve(w, r, origin(r)+"/h/"+token, token, rest, tailx.Resolver)

	case p == "/logo.svg" || p == "/favicon.svg":
		asset(w, "image/svg+xml", web.LogoSVG)
	case p == "/apple-touch-icon.png" || p == "/favicon.ico":
		asset(w, "image/png", web.TouchIcon)
	case p == "/privacy" || p == "/terms" || p == "/support":
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Write([]byte(map[string]string{"/privacy": web.Privacy, "/terms": web.Terms, "/support": web.Support}[p]))
	case p == "/.well-known/openai-apps-challenge":
		if t := strings.TrimSpace(os.Getenv("OPENAI_APPS_CHALLENGE")); t != "" {
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.Write([]byte(t))
			return
		}
		writeJSON(w, 404, map[string]string{"error": "not found"})
	case p == "/mcp":
		mcpx.ServeBearer(w, r, tailx.Resolver, oauthx.MetadataURL(origin(r)))
	case p == "/.well-known/oauth-protected-resource" || strings.HasPrefix(p, "/.well-known/oauth-protected-resource/"):
		oauthx.ProtectedResource(w, r, origin(r))
	case p == "/.well-known/oauth-authorization-server" || strings.HasPrefix(p, "/.well-known/oauth-authorization-server/") || p == "/.well-known/openid-configuration":
		oauthx.AuthServer(w, r, origin(r))
	case p == "/oauth/register":
		oauthx.Register(w, r)
	case p == "/oauth/token":
		oauthx.Token(w, r)
	case p == "/oauth/authorize" && r.Method == http.MethodGet:
		authorize(w, r)
	case p == "/api/authorize" && r.Method == http.MethodPost:
		authorizeExisting(w, r)

	case strings.HasPrefix(p, "/mcp/"):
		mcpx.Serve(w, r, strings.TrimPrefix(p, "/mcp/"), tailx.Resolver)

	case p == "/api/status":
		writeJSON(w, 200, map[string]any{"sealSecretConfigured": adbx.SealSecretConfigured()})

	case p == "/api/connect" && r.Method == http.MethodPost:
		connect(w, r)
	case p == "/api/pair" && r.Method == http.MethodPost:
		pair(w, r)
	case p == "/api/port" && r.Method == http.MethodPost:
		port(w, r)

	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

func connect(w http.ResponseWriter, r *http.Request) {
	if !adbx.SealSecretConfigured() {
		fail(w, 503, errors.New("this deployment has no SEAL_SECRET set"))
		return
	}
	var b struct {
		AuthKey string `json:"authKey"`
		State   string `json:"state"`
	}
	if readBody(r, &b) != nil {
		fail(w, 400, errors.New("invalid request"))
		return
	}
	var state []byte
	var err error
	if strings.TrimSpace(b.State) != "" {
		if state, err = decodeState(b.State); err != nil {
			fail(w, 400, err)
			return
		}
	} else if state, err = tailx.Init(r.Context(), b.AuthKey); err != nil {
		fail(w, 400, err)
		return
	}
	devs, err := tailx.Devices(r.Context(), state)
	if err != nil {
		fail(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"state": base64.StdEncoding.EncodeToString(state), "devices": devs})
}

func pair(w http.ResponseWriter, r *http.Request) {
	var b struct {
		State       string `json:"state"`
		Host        string `json:"host"`
		PairPort    int    `json:"pairPort"`
		Code        string `json:"code"`
		ConnectPort int    `json:"connectPort"`
		OAuth       string `json:"oauth"`
	}
	if readBody(r, &b) != nil || b.Host == "" || b.PairPort <= 0 || strings.TrimSpace(b.Code) == "" || b.ConnectPort <= 0 {
		fail(w, 400, errors.New("host, pairPort, code and connectPort are required"))
		return
	}
	if !tailx.AllowedHost(b.Host) {
		fail(w, 403, errors.New("host is not a tailnet address or MagicDNS name"))
		return
	}
	var authz *oauthx.Request
	if b.OAuth != "" {
		var err error
		if authz, err = oauthx.OpenRequest(b.OAuth); err != nil {
			fail(w, 400, err)
			return
		}
	}
	state, err := decodeState(b.State)
	if err != nil {
		fail(w, 400, err)
		return
	}
	dial, err := tailx.Resolver(r.Context(), state)
	if err != nil {
		fail(w, 502, err)
		return
	}
	token, _, connectMsg, err := adbx.Pair(r.Context(), dial, state, b.Host, b.PairPort, strings.TrimSpace(b.Code), b.ConnectPort)
	if err != nil {
		fail(w, 502, err)
		return
	}
	if authz != nil {
		redirect, err := authz.Complete(token)
		if err != nil {
			fail(w, 502, err)
			return
		}
		writeJSON(w, 200, map[string]any{"redirect": redirect})
		return
	}
	resp := map[string]any{"url": origin(r) + "/mcp/" + token, "httpUrl": origin(r) + "/h/" + token}
	if !strings.Contains(connectMsg, "connected to") {
		resp["warning"] = "Paired, but the first connection failed: " + connectMsg
	}
	writeJSON(w, 200, resp)
}

func port(w http.ResponseWriter, r *http.Request) {
	var b struct {
		URL  string `json:"url"`
		Port int    `json:"port"`
	}
	if readBody(r, &b) != nil || b.Port <= 0 {
		fail(w, 400, errors.New("url and port are required"))
		return
	}
	u, err := url.Parse(strings.TrimSpace(b.URL))
	token := ""
	if err == nil && strings.HasPrefix(strings.TrimRight(u.Path, "/"), "/mcp/") {
		token = strings.TrimPrefix(strings.TrimRight(u.Path, "/"), "/mcp/")
	}
	if token == "" || adbx.CheckToken(token) != nil {
		fail(w, 400, errors.New("that is not an MCP URL from this service (or it was made with a different SEAL_SECRET)"))
		return
	}
	nt, _, err := adbx.Reconnect(r.Context(), tailx.Resolver, token, "", b.Port)
	if err != nil {
		fail(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"url": origin(r) + "/mcp/" + nt, "httpUrl": origin(r) + "/h/" + nt})
}

// authorize shows the normal home page in OAuth mode. The validated request is
// embedded sealed, so the page cannot change where the user is sent back to.
func authorize(w http.ResponseWriter, r *http.Request) {
	if !adbx.SealSecretConfigured() {
		http.Error(w, "this deployment has no SEAL_SECRET set", http.StatusServiceUnavailable)
		return
	}
	q, err := oauthx.ParseAuthorize(r)
	if err != nil {
		http.Error(w, "Invalid authorization request: "+err.Error(), http.StatusBadRequest)
		return
	}
	sealed, err := q.Seal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	host := ""
	if u, err := url.Parse(q.Redirect); err == nil {
		host = u.Host
	}
	cfg, _ := json.Marshal(map[string]string{"ctx": sealed, "host": host, "name": q.Name, "cancel": q.Deny()})
	h := w.Header()
	h.Set("content-type", "text/html; charset=utf-8")
	h.Set("cache-control", "no-store")
	h.Set("referrer-policy", "no-referrer")
	h.Set("x-frame-options", "DENY")
	w.Write([]byte(strings.Replace(web.Page, "/*TS_OAUTH*/null", string(cfg), 1)))
}

// tokenFrom accepts a bare token, a "Bearer <token>" value, or a full tailshell
// URL (/mcp/<token> or /h/<token>/...).
func tokenFrom(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		for _, prefix := range []string{"/mcp/", "/h/"} {
			if rest, ok := strings.CutPrefix(u.Path, prefix); ok {
				token, _, _ := strings.Cut(rest, "/")
				return token
			}
		}
	}
	return raw
}

// authorizeExisting completes an OAuth request with a session the user already
// has, so a returning user does not have to pair again. An optional connect port
// updates the session first, for when wireless debugging was toggled.
func authorizeExisting(w http.ResponseWriter, r *http.Request) {
	var b struct {
		OAuth string `json:"oauth"`
		Token string `json:"token"`
		Port  int    `json:"port"`
	}
	if readBody(r, &b) != nil || strings.TrimSpace(b.Token) == "" {
		fail(w, 400, errors.New("paste a tailshell URL or token"))
		return
	}
	authz, err := oauthx.OpenRequest(b.OAuth)
	if err != nil {
		fail(w, 400, err)
		return
	}
	token := tokenFrom(b.Token)
	if err := adbx.CheckToken(token); err != nil {
		fail(w, 400, errors.New("that is not a valid tailshell URL or token, or it was made with a different SEAL_SECRET"))
		return
	}
	if b.Port > 0 {
		if token, _, err = adbx.Reconnect(r.Context(), tailx.Resolver, token, "", b.Port); err != nil {
			fail(w, 502, err)
			return
		}
	}
	redirect, err := authz.Complete(token)
	if err != nil {
		fail(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"redirect": redirect})
}

func asset(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("content-type", contentType)
	w.Header().Set("cache-control", "public, max-age=86400")
	w.Write(body)
}
