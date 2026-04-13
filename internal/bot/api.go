package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type APIClient struct {
	bot *tgbotapi.BotAPI
}

func NewAPIClient(token string) (*APIClient, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &APIClient{bot: bot}, nil
}

type Update = tgbotapi.Update

type Message = tgbotapi.Message

type CallbackQuery = tgbotapi.CallbackQuery

type Button struct {
	Text         string
	CallbackData string
}

func (a *APIClient) GetUpdates(ctx context.Context, offset int) ([]Update, error) {
	cfg := tgbotapi.NewUpdate(offset)
	cfg.Timeout = 20

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return a.bot.GetUpdates(cfg)
}

func (a *APIClient) SendMessage(ctx context.Context, chatID int64, text string, keyboard [][]Button) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if len(keyboard) > 0 {
		rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(keyboard))
		for _, row := range keyboard {
			buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(row))
			for _, btn := range row {
				buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData))
			}
			rows = append(rows, buttons)
		}
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	} else {
		msg.ReplyMarkup = quickActionsKeyboard()
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := a.bot.Send(msg)
	return err
}

func quickActionsKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.ReplyKeyboardMarkup{
		Keyboard: [][]tgbotapi.KeyboardButton{
			{
				tgbotapi.NewKeyboardButton("/busy now"),
				tgbotapi.NewKeyboardButton("/unbusy"),
			},
		},
		ResizeKeyboard: true,
	}
}

func (a *APIClient) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	cb := tgbotapi.NewCallback(callbackID, text)
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := a.bot.Request(cb)
	return err
}
