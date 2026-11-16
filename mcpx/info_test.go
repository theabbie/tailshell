package mcpx

import (
	"context"
	"testing"
)

func TestServerInfoIcons(t *testing.T) {
	info := serverInfo(context.WithValue(context.Background(), baseKey{}, "https://h.example"))
	icons, _ := info["icons"].([]map[string]any)
	if len(icons) != 2 || icons[0]["src"] != "https://h.example/apple-touch-icon.png" || icons[1]["src"] != "https://h.example/logo.svg" {
		t.Errorf("icons: %v", info["icons"])
	}
	if info["websiteUrl"] != "https://h.example" || info["title"] != "tailshell" {
		t.Errorf("info: %v", info)
	}
	if _, ok := serverInfo(context.Background())["icons"]; ok {
		t.Error("icons without a base URL")
	}
}
