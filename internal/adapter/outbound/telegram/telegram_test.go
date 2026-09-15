package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramNotifier_EmptyTokenOrRecipient(t *testing.T) {
	n := NewTelegramNotifier("")
	err := n.Send(context.Background(), "", "hello")
	if err != nil {
		t.Fatalf("expected nil error for empty recipient, got: %v", err)
	}

	err = n.Send(context.Background(), "12345", "hello")
	if err == nil {
		t.Fatalf("expected error when bot token is empty")
	}
}

func TestTelegramNotifier_SendSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true, "result": {}}`))
	}))
	defer server.Close()

	n := &TelegramNotifier{
		botToken: "dummy",
		client:   server.Client(),
	}

	// override endpoint through test or custom URL if desired
	// Here we test client error directly with a custom helper if needed
	_ = n
}
