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
	h.Set("Access-Control-Allow-Headers", "content-type, accept, mcp-protocol-version, mcp-session-id")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
}

func Serve(w http.ResponseWriter, r *http.Request, token string, resolve adbx.DialResolver) {
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
	for _, q := range reqs {
		if len(q.ID) == 0 {
			continue
		}
		out = append(out, handle(r.Context(), q, token, resolve))
	}
	if len(out) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeResp(w, out, batch)
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
			"annotations": map[string]any{"title": t.Title, "readOnlyHint": t.ReadOnly, "destructiveHint": !t.ReadOnly, "openWorldHint": false},
		})
	}
	return out
}

func handle(ctx context.Context, q rpcReq, token string, resolve adbx.DialResolver) rpcResp {
	resp := rpcResp{JSONRPC: "2.0", ID: q.ID}
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
			"serverInfo":      map[string]string{"name": "tailshell", "version": "1.0.0"},
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
			return resp
		}
		resp.Result = toMCP(phone.Call(ctx, token, resolve, p.Name, p.Arguments))
	default:
		resp.Error = &rpcErr{-32601, "method not found: " + q.Method}
	}
	return resp
}

func toMCP(r phone.Result) map[string]any {
	if r.Image != nil && !r.IsError {
		return map[string]any{"content": []any{map[string]string{"type": "image", "data": base64.StdEncoding.EncodeToString(r.Image), "mimeType": "image/png"}}}
	}
	return map[string]any{"content": []any{map[string]string{"type": "text", "text": r.Text}}, "isError": r.IsError}
}
