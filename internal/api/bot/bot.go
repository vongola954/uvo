package bot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"uvo/internal/clients"
	"uvo/internal/services"
)

type Bot struct {
	max     *clients.MAXClient
	gen     *services.GenerationService
	limiter *services.RateLimiter
	credits *services.CreditService
	logins  *services.LoginCodeStore
	states  sync.Map
	marker  *int64
	stop    chan struct{}
	webURL  string
}

func New(max *clients.MAXClient, gen *services.GenerationService, lim *services.RateLimiter, credits *services.CreditService, logins *services.LoginCodeStore) *Bot {
	web := strings.TrimRight(os.Getenv("WEB_PUBLIC_URL"), "/")
	if web == "" {
		web = "http://127.0.0.1:8010"
	}
	return &Bot{max: max, gen: gen, limiter: lim, credits: credits, logins: logins, stop: make(chan struct{}), webURL: web}
}

func (b *Bot) studioURL(path string) string {
	if path == "" {
		return b.webURL + "/"
	}
	return b.webURL + path
}

func (b *Bot) sendHome(chatID, userID int64, extra string) {
	text := fmt.Sprintf(
		"UVO — AI-студия музыки\n\n"+
			"%d песни в подарок · 1 кредит = 1 песня\n"+
			"Пакеты от 99₽ · кавер / караоке / свой голос\n\n"+
			"Открой студию кнопкой «Запуск» или «Открыть в MAX» ниже.\n"+
			"/generate · /credits · /help",
		services.FreeCredits,
	)
	if extra != "" {
		text += "\n\n" + extra
	}
	if err := b.max.SendStudioTo(userID, chatID, text, b.studioURL("/")); err != nil {
		logrus.WithError(err).Warn("sendHome failed")
	}
}

func (b *Bot) StartPolling() {
	if !b.max.Enabled() {
		logrus.Warn("MAX bot disabled: no token")
		return
	}
	// Fallback username from env if /me is slow/fails (open_app needs it).
	if u := strings.TrimPrefix(strings.TrimSpace(os.Getenv("MAX_BOT_USERNAME")), "@"); u != "" {
		b.max.SetUsername(u)
	}
	if me, err := b.max.Me(); err != nil {
		logrus.WithError(err).Warn("MAX /me failed — проверьте MAX_BOT_TOKEN и platform-api2.max.ru")
	} else {
		logrus.WithFields(logrus.Fields{"me": me, "startapp": b.max.StartAppURL()}).Info("MAX bot online")
	}
	// Webhook subscriptions steal updates from long-poll — clear them in polling mode.
	if subs, err := b.max.ListSubscriptions(); err != nil {
		logrus.WithError(err).Warn("MAX list subscriptions failed")
	} else if len(subs) > 0 {
		logrus.WithField("n", len(subs)).Warn("clearing webhook subscriptions so polling gets bot_started")
		if err := b.max.ClearSubscriptions(); err != nil {
			logrus.WithError(err).Warn("MAX clear subscriptions failed")
		}
	}
	if err := b.max.SetCommands([]map[string]string{
		{"name": "start", "description": "Открыть UVO / Запуск"},
		{"name": "help", "description": "Помощь"},
		{"name": "credits", "description": "Баланс кредитов"},
		{"name": "generate", "description": "Сгенерировать трек"},
		{"name": "login", "description": "Ссылка входа в студию"},
		{"name": "studio", "description": "Веб-студия"},
	}); err != nil {
		logrus.WithError(err).Warn("MAX set commands failed")
	} else {
		logrus.Info("MAX slash commands registered")
	}
	logrus.WithFields(logrus.Fields{
		"web": b.webURL, "mode": "polling", "startapp": b.max.StartAppURL(), "bot": b.max.Username(),
	}).Info("MAX long-poll + studio CTA")

	backoff := 3 * time.Second
	for {
		select {
		case <-b.stop:
			return
		default:
		}
		resp, err := b.max.GetUpdates(b.marker, 30)
		if err != nil {
			// Include err in message — Amvera often drops structured fields.
			logrus.Warnf("MAX poll error: %v", err)
			time.Sleep(backoff)
			if backoff < 60*time.Second {
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
			continue
		}
		backoff = 3 * time.Second
		if resp.Marker != nil {
			b.marker = resp.Marker
		}
		if len(resp.Updates) == 0 {
			continue
		}
		logrus.WithField("n", len(resp.Updates)).Info("poll: got updates")
		for _, u := range resp.Updates {
			b.dispatch(u)
		}
	}
}

func (b *Bot) dispatch(u clients.MAXUpdate) {
	switch u.UpdateType {
	case "bot_started":
		chatID, userID := resolveChatUser(u)
		logrus.WithFields(logrus.Fields{"chat_id": chatID, "user_id": userID, "payload": u.Payload}).Info("bot_started")
		if chatID == 0 && userID == 0 {
			logrus.Warn("bot_started without chat_id/user_id — cannot reply")
			return
		}
		b.sendHome(chatID, userID, "Добро пожаловать!")
	case "message_created":
		text, chatID, userIDStr, userID := extractMessage(u)
		if text == "" {
			return
		}
		b.HandleText(userIDStr, text, chatID, userID)
	default:
		logrus.WithField("type", u.UpdateType).Debug("MAX update ignored")
	}
}

func resolveChatUser(u clients.MAXUpdate) (chatID, userID int64) {
	chatID = u.ChatID
	userID = u.UserID
	if u.User != nil {
		if id := u.User.ID64(); id != 0 {
			userID = id
		}
	}
	if u.Message != nil {
		if u.Message.ChatID != 0 {
			chatID = u.Message.ChatID
		}
		if u.Message.Recipient != nil {
			if u.Message.Recipient.ChatID != 0 {
				chatID = u.Message.Recipient.ChatID
			}
			if userID == 0 && u.Message.Recipient.UserID != 0 {
				userID = u.Message.Recipient.UserID
			}
		}
		if u.Message.Sender != nil && userID == 0 {
			userID = u.Message.Sender.ID64()
		}
	}
	// Dialog fallback: MAX often uses the same id for private chat and user.
	if chatID == 0 && userID != 0 {
		chatID = userID
	}
	if userID == 0 && chatID != 0 {
		userID = chatID
	}
	return chatID, userID
}

func extractMessage(u clients.MAXUpdate) (text string, chatID int64, userIDStr string, userID int64) {
	chatID, userID = resolveChatUser(u)
	userIDStr = "max_user"
	if userID != 0 {
		userIDStr = strconv.FormatInt(userID, 10)
	}
	if u.Message != nil {
		if u.Message.Body != nil && u.Message.Body.Text != "" {
			text = u.Message.Body.Text
		} else if u.Message.Text != "" {
			text = u.Message.Text
		}
	}
	return strings.TrimSpace(text), chatID, userIDStr, userID
}

func (b *Bot) HandleText(userIDStr, text string, chatID, userID int64) {
	low := strings.ToLower(strings.TrimSpace(text))
	for _, p := range []string{"❓ ", "⚡ ", "/"} {
		low = strings.TrimPrefix(low, p)
	}
	switch {
	case low == "start" || low == "старт" || low == "запуск":
		b.sendHome(chatID, userID, "")
	case low == "help":
		_ = b.max.SendStudioTo(userID, chatID,
			"Команды:\n«Запуск» — веб-студия в MAX\n/generate — трек в чате (−1)\n/credits — баланс\n/login — ссылка с автологином\n\n"+
				fmt.Sprintf("%d бесплатно · пакеты от 99₽.", services.FreeCredits),
			b.studioURL("/"))
	case low == "credits" || low == "balance":
		bal := 0
		if b.credits != nil {
			bal = b.credits.Balance(userIDStr)
		}
		_ = b.max.SendStudioTo(userID, chatID,
			fmt.Sprintf("Баланс: %d кредитов\n1 кредит = 1 песня · кавер/караоке/клон −2\nПополнить: «Запуск» → Кредиты", bal),
			b.studioURL("/#pricing"))
	case low == "login" || low == "web" || low == "auth":
		b.sendLoginLink(chatID, userID, userIDStr, "/")
	case low == "studio" || strings.Contains(low, "студи"):
		b.sendHome(chatID, userID, "Открой «Запуск» ниже.")
	case low == "tracks":
		b.sendLoginLink(chatID, userID, userIDStr, "/tracks.html")
	case low == "feed":
		b.sendLoginLink(chatID, userID, userIDStr, "/feed.html")
	case low == "playlists":
		b.sendLoginLink(chatID, userID, userIDStr, "/playlists.html")
	case low == "generate" || strings.Contains(low, "generate"):
		b.states.Store(userIDStr, "await_prompt")
		_ = b.max.SendMessageToUser(userID, chatID, "Опиши трек одним сообщением (жанр, настроение, тема).\nСтоимость: −1 кредит.\n\nИли «Запуск» → режимы Идея / Текст / Instrumental.")
	default:
		if st, ok := b.states.Load(userIDStr); ok && st == "await_prompt" {
			b.states.Delete(userIDStr)
			if b.credits != nil {
				if err := b.credits.Spend(userIDStr, 1); err != nil {
					_ = b.max.SendMessageToUser(userID, chatID, "Нет кредитов: "+err.Error())
					return
				}
			}
			if err := b.limiter.Allow(userIDStr); err != nil {
				if b.credits != nil {
					b.credits.Refund(userIDStr, 1)
				}
				_ = b.max.SendMessageToUser(userID, chatID, "Лимит: "+err.Error())
				return
			}
			_ = b.max.SendMessageToUser(userID, chatID, "Генерирую… 1–3 минуты")
			go func() {
				track, err := b.gen.Generate(&services.GenerateRequest{
					UserID: userIDStr, Prompt: text, Duration: 180,
				})
				if err != nil {
					if b.credits != nil {
						b.credits.Refund(userIDStr, 1)
					}
					msg := "Операция не удалась. Попробуйте позже."
					if pe := clients.AsProviderError(err); pe != nil {
						msg = pe.Message
					}
					_ = b.max.SendMessageToUser(userID, chatID, "Ошибка: "+msg)
					return
				}
				_ = b.max.SendStudioTo(userID, chatID,
					fmt.Sprintf("Готово: %s (id %d)\nСлушай в студии («Запуск»):", track.Title, track.ID),
					b.studioURL("/tracks.html"))
			}()
			return
		}
		b.sendHome(chatID, userID, "Не понял. /help · «Запуск» · /generate")
	}
}

func (b *Bot) sendLoginLink(chatID, userID int64, userIDStr, path string) {
	if b.logins == nil {
		_ = b.max.SendStudioTo(userID, chatID, "Студия:", b.studioURL(path))
		return
	}
	code, err := b.logins.Issue(userIDStr, 15*time.Minute)
	if err != nil {
		_ = b.max.SendMessageToUser(userID, chatID, "Не удалось выдать код входа")
		return
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	url := b.studioURL(path) + sep + "code=" + code
	_ = b.max.SendStudioTo(userID, chatID, "Одноразовая ссылка (15 мин) или кнопка «Запуск»:", url)
}

func (b *Bot) HandleWebhookUpdate(u clients.MAXUpdate) {
	b.dispatch(u)
}
