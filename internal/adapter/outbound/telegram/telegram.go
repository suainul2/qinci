package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"qinci/internal/core/ports"
)

type TelegramNotifier struct {
	botToken string
	client   *http.Client
}

var _ ports.Notifier = (*TelegramNotifier)(nil)

func NewTelegramNotifier(botToken string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: strings.TrimSpace(botToken),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendMessagePayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

func (t *TelegramNotifier) Send(ctx context.Context, recipient, message string) error {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return nil
	}
	if t.botToken == "" {
		log.Println("[TELEGRAM] Bot token tidak disetel. Melewati pengiriman notifikasi.")
		return errors.New("telegram bot token tidak dikonfigurasi di server")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	reqBody := sendMessagePayload{
		ChatID:    recipient,
		Text:      message,
		ParseMode: "HTML",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("gagal encode payload telegram: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("gagal membuat request telegram: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal menghubungi telegram API: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var teleResp telegramResponse
	if err := json.Unmarshal(respBytes, &teleResp); err != nil {
		return fmt.Errorf("gagal parse response telegram (status %d)", resp.StatusCode)
	}

	if !teleResp.OK {
		return fmt.Errorf("telegram API error: %s", teleResp.Description)
	}

	return nil
}
