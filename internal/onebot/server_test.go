package onebot

import (
	"context"
	"net/http"
	"testing"

	"milky-onebot11-bridge/internal/config"
)

type stubHandler struct{}

func (stubHandler) HandleAPI(context.Context, APIRequest) APIResponse { return Success(nil, nil) }
func (stubHandler) OnWSConnect(context.Context, string) []any         { return nil }
func (stubHandler) CurrentSelfID() int64                              { return 123456 }

func TestNormalizeHTTPScalar(t *testing.T) {
	if got, ok := normalizeHTTPScalar("user_id", "123").(int64); !ok || got != 123 {
		t.Fatalf("expected int64 user_id, got %#v", normalizeHTTPScalar("user_id", "123"))
	}
	if got, ok := normalizeHTTPScalar("approve", "true").(bool); !ok || !got {
		t.Fatalf("expected bool approve, got %#v", normalizeHTTPScalar("approve", "true"))
	}
	if got, ok := normalizeHTTPScalar("message", "123").(string); !ok || got != "123" {
		t.Fatalf("expected string message, got %#v", normalizeHTTPScalar("message", "123"))
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "a", "b"); got != "a" {
		t.Fatalf("expected first non-empty value, got %q", got)
	}
}

func TestAuthorized(t *testing.T) {
	server := NewServer(config.OneBotConfig{AccessToken: "abc"}, nil, stubHandler{})
	req, err := http.NewRequest(http.MethodGet, "http://example.com/?access_token=abc", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if !server.authorized(req) {
		t.Fatalf("expected authorized request")
	}
}
