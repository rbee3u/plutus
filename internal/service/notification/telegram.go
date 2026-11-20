package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/httpclient"
)

const (
	telegramAPIBaseURL = "https://api.telegram.org"
	telegramTimeout    = 10 * time.Second
	maxTelegramBody    = 1 << 20
)

// Sender delivers one already-rendered message.
type Sender interface {
	Send(ctx context.Context, text string) error
}

// TelegramOptions configures the Bot API sender. A nil Client builds the
// standard IPv4 client, optionally through ProxyURL.
type TelegramOptions struct {
	BotToken string
	ChatID   string
	ProxyURL *url.URL
	Client   *http.Client
	BaseURL  string
}

// TelegramSender sends messages through the Telegram Bot API.
type TelegramSender struct {
	client   *http.Client
	baseURL  string
	botToken string
	chatID   string
}

// NewTelegramSender validates the bot settings and constructs a sender.
func NewTelegramSender(opts TelegramOptions) (*TelegramSender, error) {
	botToken := strings.TrimSpace(opts.BotToken)
	chatID := strings.TrimSpace(opts.ChatID)
	if botToken == "" || chatID == "" {
		return nil, errors.New("telegram bot token and chat id are required")
	}
	client := opts.Client
	if client == nil {
		if opts.ProxyURL != nil {
			proxy := *opts.ProxyURL
			client = httpclient.NewIPv4WithProxy(telegramTimeout, func(*http.Request) (*url.URL, error) {
				return &proxy, nil
			})
		} else {
			client = httpclient.NewIPv4(telegramTimeout)
		}
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = telegramAPIBaseURL
	}
	return &TelegramSender{client: client, baseURL: baseURL, botToken: botToken, chatID: chatID}, nil
}

// Send posts one plain-text message. Errors never include the bot token.
func (s *TelegramSender) Send(ctx context.Context, text string) error {
	payload, err := json.Marshal(struct {
		ChatID                string `json:"chat_id"`
		Text                  string `json:"text"`
		DisableWebPagePreview bool   `json:"disable_web_page_preview"`
	}{ChatID: s.chatID, Text: text, DisableWebPagePreview: true})
	if err != nil {
		return fmt.Errorf("encode telegram message: %w", err)
	}
	// The bot token is part of the URL path. Never wrap or log the request URL.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/bot"+s.botToken+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return errors.New("build telegram request failed")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return telegramRequestError(err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, maxTelegramBody))
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(raw, &result)
	if response.StatusCode != http.StatusOK || !result.OK {
		description := strings.TrimSpace(result.Description)
		if description == "" {
			description = response.Status
		}
		return fmt.Errorf("telegram send failed: %s", description)
	}
	return nil
}

// telegramRequestError strips every url.Error wrapper, because the URL
// contains the token and a transport may nest another url.Error inside it.
func telegramRequestError(err error) error {
	for {
		var urlErr *url.Error
		if !errors.As(err, &urlErr) {
			break
		}
		if urlErr.Err == nil {
			return errors.New("telegram request failed")
		}
		err = urlErr.Err
	}
	if err == nil {
		return errors.New("telegram request failed")
	}
	return fmt.Errorf("telegram request failed: %w", err)
}
