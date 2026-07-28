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
	"uvo/internal/middleware"
	"uvo/internal/services"
)

type Bot struct {
	max       *clients.MAXClient
	gen       *services.GenerationService
	limiter   *services.RateLimiter
	credits   *services.CreditService
	states    sync.Map
	marker    *int64
	stop      chan struct{}
	webURL    string
	jwtSecret string
}

func New(max *clients.MAXClient, gen *services.GenerationService, lim *services.RateLimiter, credits *services.CreditService, jwtSecret string) *Bot {
	web := strings.TrimRight(os.Getenv("WEB_PUBLIC_URL"), "/")
	if web == "" {
		web = "http://127.0.0.1:8010"
	}
	return &Bot{max: max, gen: gen, limiter: lim, credits: credits, stop: make(chan struct{}), webURL: web, jwtSecret: jwtSecret}
}

func (b *Bot) studioURL(path string) string {
	if path == "" {
		return b.webURL + "/"
	}
	return b.webURL + path
}

func (b *Bot) sendHome(chatID int64, extra string) {
	text := "UVO — AI-музыка\n\nВеб-студия и генерация в одном месте."
	if extra != "" {
		text += "\n\n" + extra
	}
	_ = b.max.SendStudio(chatID, text, b.studioURL("/"))
}

func (b *Bot) StartPolling() {
	if !b.max.Enabled() {
		logrus.Warn("MAX bot disabled: no token")
		return
	}
	if me, err := b.max.Me(); err != nil {
		logrus.WithError(err).Warn("MAX /me failed")
	} else {
		logrus.WithField("me", me).Info("MAX bot online")
	}
	logrus.WithField("web", b.webURL).Info("MAX long-poll + web studio link")

	for {
		select {
		case <-b.stop:
			return
		default:
		}
		resp, err := b.max.GetUpdates(b.marker, 30)
		if err != nil {
			logrus.WithError(err).Warn("MAX poll error")
			time.Sleep(3 * time.Second)
			continue
		}
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
		chatID := u.ChatID
		b.sendHome(chatID, "Нажми кнопку студии или /generate")
	case "message_created":
		text, chatID, userID := extractMessage(u)
		if text == "" {
			return
		}
		b.HandleText(userID, text, chatID)
	}
}

func extractMessage(u clients.MAXUpdate) (text string, chatID int64, userID string) {
	chatID = u.ChatID
	userID = "max_user"
	if u.User != nil {
		userID = strconv.FormatInt(u.User.UserID, 10)
	}
	if u.Message != nil {
		if u.Message.ChatID != 0 {
			chatID = u.Message.ChatID
		}
		if u.Message.Sender != nil {
			userID = strconv.FormatInt(u.Message.Sender.UserID, 10)
		}
		if u.Message.Body != nil && u.Message.Body.Text != "" {
			text = u.Message.Body.Text
		} else if u.Message.Text != "" {
			text = u.Message.Text
		}
	}
	return strings.TrimSpace(text), chatID, userID
}

func (b *Bot) HandleText(userID, text string, chatID int64) {
	// message buttons may send "/generate" as text
	low := strings.TrimSpace(text)
	switch {
	case low == "/start" || low == "start":
		b.sendHome(chatID, "Войти в веб: /login")
	case low == "/help" || low == "❓ /help":
		_ = b.max.SendStudio(chatID,
			"Команды:\n/generate — трек в чате\n/login — вход в веб-студию\n/studio — ссылка\n/tracks — мои треки\n\nПолный UI — после /login.",
			b.studioURL("/"))
	case low == "/login" || low == "/web" || low == "/auth":
		b.sendLoginLink(chatID, userID)
	case low == "/studio" || strings.Contains(low, "студи"):
		b.sendLoginLink(chatID, userID)
	case low == "/tracks":
		b.sendLoginLinkTo(chatID, userID, "/tracks.html")
	case low == "/feed":
		b.sendLoginLinkTo(chatID, userID, "/feed.html")
	case low == "/playlists":
		b.sendLoginLinkTo(chatID, userID, "/playlists.html")
	case low == "/generate" || low == "⚡ /generate":
		b.states.Store(userID, "await_prompt")
		_ = b.max.SendMessage(chatID, "Опиши трек одним сообщением (жанр, настроение, тема):\n\nИли /login → веб-студия.")
		b.sendLoginLink(chatID, userID)
	default:
		if st, ok := b.states.Load(userID); ok && st == "await_prompt" {
			b.states.Delete(userID)
			if b.credits != nil {
				if err := b.credits.Spend(userID, 1); err != nil {
					_ = b.max.SendMessage(chatID, "Нет кредитов: "+err.Error())
					return
				}
			}
			if err := b.limiter.Allow(userID); err != nil {
				if b.credits != nil {
					b.credits.Refund(userID, 1)
				}
				_ = b.max.SendMessage(chatID, "Лимит: "+err.Error())
				return
			}
			_ = b.max.SendMessage(chatID, "Генерирую… 1–3 минуты")
			go func() {
				track, err := b.gen.Generate(&services.GenerateRequest{
					UserID: userID, Prompt: text, Duration: 180,
				})
				if err != nil {
					if b.credits != nil {
						b.credits.Refund(userID, 1)
					}
					msg := "Операция не удалась. Попробуйте позже."
					if pe := clients.AsProviderError(err); pe != nil {
						msg = pe.Message
					}
					_ = b.max.SendMessage(chatID, "Ошибка: "+msg)
					return
				}
				play := b.studioURL("/tracks.html")
				_ = b.max.SendStudio(chatID,
					fmt.Sprintf("Готово: %s (id %d)\nСлушай в веб-студии (/login):", track.Title, track.ID),
					play)
			}()
			return
		}
		b.sendHome(chatID, "Не понял. /help · /login · /generate")
	}
}

func (b *Bot) sendLoginLink(chatID int64, userID string) {
	b.sendLoginLinkTo(chatID, userID, "/")
}

func (b *Bot) sendLoginLinkTo(chatID int64, userID, path string) {
	if b.jwtSecret == "" {
		_ = b.max.SendStudio(chatID, "Студия (без автологина):", b.studioURL(path))
		return
	}
	tok, err := middleware.IssueToken(b.jwtSecret, userID, 7*24*time.Hour)
	if err != nil {
		_ = b.max.SendMessage(chatID, "Не удалось выдать сессию")
		return
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	url := b.studioURL(path) + sep + "token=" + tok
	_ = b.max.SendStudio(chatID, "Вход в UVO (ссылка на 7 дней, только для вас):", url)
}

func (b *Bot) HandleWebhookUpdate(u clients.MAXUpdate) {
	b.dispatch(u)
}
