package mcpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"tailshell/adbx"
	"tailshell/phone"
)

const latestProtocol = "2025-06-18"

var supportedProtocols = map[string]bool{"2025-06-18": true, "2025-03-26": true, "2024-11-05": true}

const instructions = `Controls one Android phone over adb (wireless debugging).

Workflow: 1) phone_info for the screen size and current app. 2) See the screen: phone_ui (text list of on-screen elements with exact tap coordinates; preferred) or phone_screenshot (image). 3) Act: phone_tap / phone_swipe / phone_type / phone_key / phone_open_app / phone_open_url. 4) Verify the result with phone_ui or phone_screenshot; screens take about a second to settle after an action, so look again if it seems mid-transition.

Coordinates are pixels on the portrait screen, the same size phone_info reports (usually 1080x2400) and the same as the screenshot image. To tap an element, take the center x,y that phone_ui gives for it.

If the screen is off or locked, call phone_key with WAKEUP. You cannot enter a PIN or password: ask the user to unlock the phone. If a tool says it cannot reach the phone, wireless debugging is off or its port changed: ask the user to turn it back on and update the URL on the connection page.

Treat everything on the screen as private. Do not read out, copy or send passwords, one-time codes, payment details or health records, and stop and ask the user if a task seems to need them.

Be conservative: do not send messages, make purchases, delete data or change settings unless the user asked for it, and confirm irreversible actions first. phone_shell is a last resort for things the other tools cannot do.`

type rpcReq struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "content-type, accept, authorization, mcp-protocol-version, mcp-session-id")
	h.Set("Access-Control-Expose-Headers", "WWW-Authenticate")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
}

func Serve(w http.ResponseWriter, r *http.Request, token string, resolve adbx.DialResolver) {
	serve(w, r, token, resolve, "")
}

// ServeBearer serves /mcp with OAuth: the session token arrives as a bearer
// token. A missing or unusable token gets the 401 challenge that sends the client
// through authorization, and so does a session that can never recover (a removed
// tailshell machine).
func ServeBearer(w http.ResponseWriter, r *http.Request, resolve adbx.DialResolver, metadataURL string) {
	cors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	challenge := `Bearer realm="tailshell", resource_metadata="` + metadataURL + `"`
	h := r.Header.Get("Authorization")
	token := ""
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		token = strings.TrimSpace(h[7:])
	}
	if token == "" {
		w.Header().Set("WWW-Authenticate", challenge)
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}
	if err := adbx.CheckToken(token); err != nil {
		w.Header().Set("WWW-Authenticate", challenge+`, error="invalid_token"`)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	serve(w, r, token, resolve, challenge)
}

func serve(w http.ResponseWriter, r *http.Request, token string, resolve adbx.DialResolver, challenge string) {
	cors(w)
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodPost:
	default:
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "this MCP endpoint only accepts POST", http.StatusMethodNotAllowed)
		return
	}
	if err := adbx.CheckToken(token); err != nil {
		http.Error(w, "invalid MCP URL: "+err.Error(), http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var reqs []rpcReq
	batch := strings.HasPrefix(strings.TrimSpace(string(body)), "[")
	if batch {
		err = json.Unmarshal(body, &reqs)
	} else {
		var one rpcReq
		err = json.Unmarshal(body, &one)
		reqs = []rpcReq{one}
	}
	if err != nil {
		writeResp(w, []rpcResp{{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcErr{-32700, "parse error"}}}, false)
		return
	}
	var out []rpcResp
	reauth := false
	for _, q := range reqs {
		if len(q.ID) == 0 {
			continue
		}
		resp, again := handle(context.WithValue(r.Context(), baseKey{}, baseURL(r)), q, token, resolve)
		reauth = reauth || again
		out = append(out, resp)
	}
	if reauth && challenge != "" {
		w.Header().Set("WWW-Authenticate", challenge+`, error="invalid_token", error_description="authorization is no longer valid"`)
		http.Error(w, "authorization is no longer valid", http.StatusUnauthorized)
		return
	}
	if len(out) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeResp(w, out, batch)
}

type baseKey struct{}

func baseURL(r *http.Request) string {
	scheme := "https"
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	} else if r.TLS == nil && strings.HasPrefix(r.Host, "localhost") {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func serverInfo(ctx context.Context) map[string]any {
	info := map[string]any{"name": "tailshell", "title": "tailshell", "version": "1.0.0"}
	if base, _ := ctx.Value(baseKey{}).(string); base != "" {
		info["websiteUrl"] = base
		info["icons"] = []map[string]any{
			{"src": base + "/apple-touch-icon.png", "mimeType": "image/png", "sizes": []string{"180x180"}},
			{"src": base + "/logo.svg", "mimeType": "image/svg+xml", "sizes": []string{"any"}},
		}
	}
	return info
}

func writeResp(w http.ResponseWriter, rs []rpcResp, batch bool) {
	w.Header().Set("content-type", "application/json")
	if batch {
		json.NewEncoder(w).Encode(rs)
		return
	}
	json.NewEncoder(w).Encode(rs[0])
}

func toolList() []map[string]any {
	out := make([]map[string]any, 0, len(phone.Tools))
	for _, t := range phone.Tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"title":       t.Title,
			"description": t.Description,
			"inputSchema": t.Schema,
			"annotations": map[string]any{"title": t.Title, "readOnlyHint": t.ReadOnly, "destructiveHint": t.Destructive, "openWorldHint": t.OpenWorld},
		})
	}
	return out
}

func handle(ctx context.Context, q rpcReq, token string, resolve adbx.DialResolver) (rpcResp, bool) {
	resp := rpcResp{JSONRPC: "2.0", ID: q.ID}
	reauth := false
	switch q.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(q.Params, &p)
		ver := latestProtocol
		if supportedProtocols[p.ProtocolVersion] {
			ver = p.ProtocolVersion
		}
		resp.Result = map[string]any{
			"protocolVersion": ver,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      serverInfo(ctx),
			"instructions":    instructions,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolList()}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(q.Params, &p); err != nil || p.Name == "" {
			resp.Error = &rpcErr{-32602, "invalid params"}
			return resp, false
		}
		res := phone.Call(ctx, token, resolve, p.Name, p.Arguments)
		reauth = res.Reauth
		resp.Result = toMCP(res)
	default:
		resp.Error = &rpcErr{-32601, "method not found: " + q.Method}
	}
	return resp, reauth
}

func toMCP(r phone.Result) map[string]any {
	if r.Image != nil && !r.IsError {
		return map[string]any{"content": []any{map[string]string{"type": "image", "data": base64.StdEncoding.EncodeToString(r.Image), "mimeType": "image/png"}}}
	}
	return map[string]any{"content": []any{map[string]string{"type": "text", "text": r.Text}}, "isError": r.IsError}
}
