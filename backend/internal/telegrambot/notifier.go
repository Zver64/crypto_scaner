package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"crypto-scanner/internal/auth"
	"crypto-scanner/internal/platform/backoff"

	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/time/rate"
)

// sendTimeout bounds the recipient lookup and the Telegram API call of a
// notification. Rate-limit waits, the bot-wide limiter's and Telegram's
// retry_after, are not bounded, so a burst is delayed, never dropped.
const sendTimeout = 10 * time.Second

// Notifier sends notifications, written in Telegram HTML, within the bot-wide
// rate limit, and retries once after a Telegram 429.
type Notifier struct {
	bot             *telegram.Bot
	users           auth.UserStore
	administratorID int64
	limiter         *rate.Limiter
}

func newNotifier(bot *telegram.Bot, users auth.UserStore, administratorID int64) *Notifier {
	return &Notifier{bot: bot, users: users, administratorID: administratorID, limiter: rate.NewLimiter(rate.Limit(25), 25)}
}

// Send delivers text to a user, such as a price or strategy alert. It
// rechecks the recipient's access immediately before sending.
func (notifier *Notifier) Send(ctx context.Context, telegramID int64, text string) error {
	return notifier.notify(ctx, telegramID, text, true)
}

// SendAdministratorMessage sends an operational message to the configured
// administrator. The administrator is configured, so access is not
// rechecked.
func (notifier *Notifier) SendAdministratorMessage(ctx context.Context, text string) error {
	return notifier.notify(ctx, notifier.administratorID, text, false)
}

// notification builds the parameters of text, written in Telegram HTML,
// without previews of its links. Every notification goes so; the bot's own
// dialog stays plain.
func notification(chatID int64, text string) *telegram.SendMessageParams {
	disabled := true
	return &telegram.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML, LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disabled}}
}

// notify sends text to chatID and, when Telegram answers 429, waits its
// retry_after with the caller's context and tries once more.
func (notifier *Notifier) notify(ctx context.Context, chatID int64, text string, recheckAccess bool) error {
	for retried := false; ; retried = true {
		err := notifier.send(ctx, chatID, text, recheckAccess)
		var tooMany *telegram.TooManyRequestsError
		if retried || !errors.As(err, &tooMany) {
			return err
		}
		if err := backoff.Sleep(ctx, time.Duration(tooMany.RetryAfter)*time.Second); err != nil {
			return err
		}
	}
}

// send waits for the bot-wide rate limit with the caller's context, so only
// its cancellation ends the wait, and then sends text, rechecking the
// recipient's access first when asked.
func (notifier *Notifier) send(ctx context.Context, chatID int64, text string, recheckAccess bool) error {
	if err := notifier.limiter.Wait(ctx); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if recheckAccess {
		if _, err := notifier.users.FindByTelegramID(ctx, chatID); errors.Is(err, auth.ErrUserNotFound) {
			return fmt.Errorf("recipient no longer has access")
		} else if err != nil {
			return fmt.Errorf("look up recipient: %w", err)
		}
	}
	_, err := notifier.bot.SendMessage(ctx, notification(chatID, text))
	return err
}
