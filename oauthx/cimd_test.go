package oauthx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCIMD(t *testing.T) {
	var doc map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()
	id := srv.URL + "/client.json"

	if _, err := fetchCIMD(t.Context(), id); err == nil {
		t.Fatal("loopback metadata host was fetched")
	}

	saved := cimdClient
	cimdClient = srv.Client()
	defer func() { cimdClient = saved }()

	doc = map[string]any{"client_id": id, "client_name": "Example", "redirect_uris": []string{"https://app.example/cb"}}
	c, err := fetchCIMD(t.Context(), id)
	if err != nil || c.Name != "Example" || c.Redirects[0] != "https://app.example/cb" {
		t.Fatalf("valid document: %v %v", c, err)
	}

	doc = map[string]any{"client_id": id, "redirect_uris": []string{"https://app.example/cb"}, "token_endpoint_auth_method": "private_key_jwt", "token_endpoint_auth_methods_supported": []string{"none", "private_key_jwt"}}
	if _, err := fetchCIMD(t.Context(), id); err != nil {
		t.Fatalf("client that also supports none: %v", err)
	}

	bad := []map[string]any{
		{"client_id": "https://other.example/c.json", "redirect_uris": []string{"https://app.example/cb"}},
		{"client_id": id, "redirect_uris": []string{"http://app.example/cb"}},
		{"client_id": id, "redirect_uris": []string{"https://app.example/cb"}, "token_endpoint_auth_method": "client_secret_basic"},
		{"client_id": id},
	}
	for i, d := range bad {
		doc = d
		if _, err := fetchCIMD(t.Context(), id); err == nil {
			t.Errorf("bad document %d accepted", i)
		}
	}
}

func TestCIMDAdvertised(t *testing.T) {
	var as map[string]any
	w := httptest.NewRecorder()
	AuthServer(w, httptest.NewRequest("GET", "/", nil), "https://h.example")
	json.Unmarshal(w.Body.Bytes(), &as)
	if as["client_id_metadata_document_supported"] != true {
		t.Errorf("CIMD not advertised: %v", as)
	}
}
