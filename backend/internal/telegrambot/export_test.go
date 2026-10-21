package telegrambot

import telegram "github.com/go-telegram/bot"

// UseHTTPClient replaces the service's Telegram transport with client, so a
// test can run the bot without a network, such as inside a synctest bubble.
func UseHTTPClient(service *Service, token string, client telegram.HttpClient) error {
	bot, err := telegram.New(token, telegram.WithHTTPClient(0, client), telegram.WithSkipGetMe())
	if err != nil {
		return err
	}
	service.bot = bot
	return nil
}
