package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTelegramSenderSendsPlainText(t *testing.T) {
	t.Parallel()

	var path string
	var payload struct {
		ChatID                string `json:"chat_id"`
		Text                  string `json:"text"`
		DisableWebPagePreview bool   `json:"disable_web_page_preview"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender, err := NewTelegramSender(TelegramOptions{
		BotToken: "123:secret",
		ChatID:   "-1001234567890",
		Client:   server.Client(),
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(context.Background(), "High leverage"); err != nil {
		t.Fatal(err)
	}
	if path != "/bot123:secret/sendMessage" {
		t.Fatalf("path: %q", path)
	}
	if payload.ChatID != "-1001234567890" || payload.Text != "High leverage" || !payload.DisableWebPagePreview {
		t.Fatalf("payload: %+v", payload)
	}
}

func TestTelegramSenderReportsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	sender, err := NewTelegramSender(TelegramOptions{
		BotToken: "123:secret",
		ChatID:   "-100",
		Client:   server.Client(),
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Send(context.Background(), "text")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("expected API error, got %v", err)
	}
	if strings.Contains(err.Error(), "123:secret") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: "Post", URL: request.URL.String(), Err: errors.New("dial failed")}
}

func TestTelegramSenderSanitizesNetworkError(t *testing.T) {
	t.Parallel()

	sender, err := NewTelegramSender(TelegramOptions{
		BotToken: "123:secret",
		ChatID:   "-100",
		Client:   &http.Client{Transport: failingTransport{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Send(context.Background(), "text")
	if err == nil || !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("expected request error, got %v", err)
	}
	if strings.Contains(err.Error(), "123:secret") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func TestNewTelegramSenderRequiresSettings(t *testing.T) {
	t.Parallel()

	for _, opts := range []TelegramOptions{
		{ChatID: "-100"},
		{BotToken: "123:secret"},
		{BotToken: " ", ChatID: "-100"},
	} {
		if _, err := NewTelegramSender(opts); err == nil {
			t.Fatalf("expected validation error for %+v", opts)
		}
	}
}
