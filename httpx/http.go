package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tailshell/adbx"
	"tailshell/phone"
)

func pathOf(t phone.Tool) string { return "/" + strings.TrimPrefix(t.Name, "phone_") }

func cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "content-type, accept")
	h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}

func Serve(w http.ResponseWriter, r *http.Request, base, token, rest string, resolve adbx.DialResolver) {
	cors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := adbx.CheckToken(token); err != nil {
		errJSON(w, http.StatusUnauthorized, "invalid API URL: "+err.Error())
		return
	}
	rest = "/" + strings.Trim(rest, "/")
	if rest == "/" {
		index(w, base)
		return
	}
	if rest == "/openapi.json" {
		writeJSON(w, http.StatusOK, openAPI(base))
		return
	}
	for _, t := range phone.Tools {
		if pathOf(t) == rest {
			run(w, r, t, token, resolve)
			return
		}
	}
	errJSON(w, http.StatusNotFound, "unknown endpoint; GET "+base+" lists them")
}

func index(w http.ResponseWriter, base string) {
	type ep struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Methods     []string       `json:"methods"`
		URL         string         `json:"url"`
		Input       map[string]any `json:"input"`
	}
	eps := make([]ep, 0, len(phone.Tools))
	for _, t := range phone.Tools {
		methods := []string{"POST"}
		if t.ReadOnly {
			methods = []string{"GET", "POST"}
		}
		eps = append(eps, ep{t.Name, t.Description, methods, base + pathOf(t), t.Schema})
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": "tailshell", "openapi": base + "/openapi.json", "endpoints": eps})
}

func run(w http.ResponseWriter, r *http.Request, t phone.Tool, token string, resolve adbx.DialResolver) {
	args := map[string]any{}
	switch r.Method {
	case http.MethodGet:
		if !t.ReadOnly {
			w.Header().Set("Allow", "POST, OPTIONS")
			errJSON(w, http.StatusMethodNotAllowed, t.Name+" changes the phone: use POST with a JSON body")
			return
		}
		props, _ := t.Schema["properties"].(map[string]any)
		for k, p := range props {
			v := r.URL.Query().Get(k)
			if v == "" {
				continue
			}
			if m, ok := p.(map[string]any); ok && m["type"] == "integer" {
				n, err := strconv.Atoi(v)
				if err != nil {
					errJSON(w, http.StatusBadRequest, k+" must be an integer")
					return
				}
				args[k] = float64(n)
				continue
			}
			args[k] = v
		}
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			errJSON(w, http.StatusBadRequest, "bad request")
			return
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &args); err != nil {
				errJSON(w, http.StatusBadRequest, "body must be a JSON object")
				return
			}
		}
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		errJSON(w, http.StatusMethodNotAllowed, "use GET or POST")
		return
	}

	res := phone.Call(r.Context(), token, resolve, t.Name, args)
	if res.Image != nil && !res.IsError {
		w.Header().Set("content-type", "image/png")
		w.Header().Set("cache-control", "no-store")
		w.Write(res.Image)
		return
	}
	if res.IsError {
		code := res.Status
		if code == 0 {
			code = http.StatusOK
		}
		body := map[string]any{"ok": false, "error": res.Text}
		writeJSON(w, code, body)
		return
	}
	body := map[string]any{"ok": true, "text": res.Text}
	if res.Data != nil {
		body["data"] = res.Data
	}
	writeJSON(w, http.StatusOK, body)
}

func openAPI(base string) map[string]any {
	errSchema := map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}, "error": map[string]any{"type": "string"}}}
	okSchema := map[string]any{"type": "object", "properties": map[string]any{
		"ok":   map[string]any{"type": "boolean"},
		"text": map[string]any{"type": "string"},
		"data": map[string]any{"type": "object", "description": "structured result when available (phone_ui: elements, phone_info: model/android/size/focus, phone_list_apps: packages)"},
	}}
	jsonResp := func(s map[string]any) map[string]any {
		return map[string]any{"application/json": map[string]any{"schema": s}}
	}
	paths := map[string]any{}
	for _, t := range phone.Tools {
		ok := map[string]any{"description": "success", "content": jsonResp(okSchema)}
		if t.Name == "phone_screenshot" {
			ok = map[string]any{"description": "PNG image of the screen", "content": map[string]any{"image/png": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}
		}
		op := map[string]any{
			"operationId": t.Name,
			"summary":     t.Title,
			"description": t.Description,
			"responses": map[string]any{
				"200": ok,
				"400": map[string]any{"description": "invalid arguments", "content": jsonResp(errSchema)},
				"502": map[string]any{"description": "phone unreachable", "content": jsonResp(errSchema)},
			},
		}
		method := "post"
		if t.ReadOnly {
			method = "get"
			var params []any
			props, _ := t.Schema["properties"].(map[string]any)
			for name, p := range props {
				params = append(params, map[string]any{"name": name, "in": "query", "required": false, "schema": p})
			}
			if params != nil {
				op["parameters"] = params
			}
		} else {
			op["requestBody"] = map[string]any{"required": true, "content": jsonResp(t.Schema)}
		}
		paths[pathOf(t)] = map[string]any{method: op}
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "tailshell phone API", "version": "1.0.0", "description": "Control one Android phone over adb. Coordinates are pixels on the portrait screen. The connection is encoded in the server URL, so no authentication is needed; keep the URL private."},
		"servers": []any{map[string]any{"url": base}},
		"paths":   paths,
	}
}
