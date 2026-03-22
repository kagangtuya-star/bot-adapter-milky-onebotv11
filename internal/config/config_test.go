package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
  "milky": {
    "ws_gateway": "ws://127.0.0.1:22345/event",
    "rest_gateway": "http://127.0.0.1:22345/api",
    "token": ""
  },
  "onebot": {
    "host": "127.0.0.1",
    "port": 6700,
    "access_token": "",
    "enable_http_api": false,
    "enable_ws_api": true,
    "enable_ws_event": true,
    "enable_ws_universal": true,
    "reverse": {
      "enable": false,
      "url": "",
      "api_url": "",
      "event_url": "",
      "use_universal_client": false,
      "reconnect_interval_ms": 3000
    }
  },
  "bridge": {
    "self_id": 123,
    "message_format": "array",
    "heartbeat_interval_ms": 15000,
    "log_level": "info",
    "cache_size": 128
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Milky.WSGateway == "" || cfg.OneBot.Port != 6700 || cfg.Bridge.SelfID != 123 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
