package telegram

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const secretHeader = "X-Telegram-Bot-Api-Secret-Token"
const maxUpdateBytes int64 = 1 << 20

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	Text string `json:"text"`
	Chat chat   `json:"chat"`
}

type chat struct {
	ID int64 `json:"id"`
}

// UpdateProcessor handles one decoded Telegram update.
type UpdateProcessor interface {
	HandleUpdate(ctx context.Context, update Update) error
}

// NewWebhookHandler returns the public Telegram update HTTP boundary.
func NewWebhookHandler(secret string, processor UpdateProcessor) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		providedHash := sha256.Sum256([]byte(request.Header.Get(secretHeader)))
		secretHash := sha256.Sum256([]byte(secret))
		if subtle.ConstantTimeCompare(providedHash[:], secretHash[:]) != 1 {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, maxUpdateBytes)
		decoder := json.NewDecoder(request.Body)
		var incoming update
		if err := decoder.Decode(&incoming); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				response.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		decoded := Update{}
		if incoming.Message != nil {
			decoded.Message = &Message{Text: incoming.Message.Text, ChatID: incoming.Message.Chat.ID}
		}
		if err := processor.HandleUpdate(request.Context(), decoded); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
}
