package app

import (
	"net/http/httptest"
	"testing"
)

func TestPackagesInitialize(t *testing.T) {
	w := httptest.NewRecorder()
	Handler(w, httptest.NewRequest("GET", "/api/status", nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
}
