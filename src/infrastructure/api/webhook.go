package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	app "github.com/ChatDetectiveORG/api-gateway/src/application"
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/config"
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/postgresql"
	"github.com/ChatDetectiveORG/api-gateway/src/infrastructure/rabbitmq"
	e "github.com/ChatDetectiveORG/shared/errors"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	sharedTelegram "github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"
)

func SetupWebhook(config *config.Config) (*tele.Bot, *e.ErrorInfo) {
	poller := &MirrorWebhookPoller{
		Listen:      ":" + config.TeleAPIWebhookConfig.Port,
		PublicURL:   config.TeleAPIWebhookConfig.URL,
		SecretToken: config.TeleAPIWebhookConfig.Secret,
	}
	pref := tele.Settings{
		Token:  config.TeleAPIWebhookConfig.Token,
		Poller: poller,
	}

	client, err := tele.NewBot(pref)
	if err != nil {
		return nil, e.FromError(err, "Failed to create bot").
			WithSeverity(e.Critical)
	}

	return client, e.Nil()
}

var allowedWebhookUpdates = []string{
	"message",
	"callback_query",
	"shipping_query",
	"pre_checkout_query",
	"business_connection",
	"business_message",
	"edited_business_message",
	"deleted_business_messages",
}

type MirrorWebhookPoller struct {
	Listen      string
	PublicURL   string
	SecretToken string
}

// addUpdate is an indirection over app.AddTelegramUpdate for tests.
var addUpdate = app.AddTelegramUpdate

func (p *MirrorWebhookPoller) Poll(b *tele.Bot, updates chan tele.Update, stop chan struct{}) {
	if err := b.SetWebhook(&tele.Webhook{
		Endpoint:       &tele.WebhookEndpoint{PublicURL: p.PublicURL},
		MaxConnections: 100,
		AllowedUpdates: allowedWebhookUpdates,
		SecretToken:    p.SecretToken,
	}); err != nil {
		b.OnError(err, nil)
		return
	}

	mux := http.NewServeMux()
	mainPath := webhookPath(p.PublicURL)
	mux.HandleFunc(mainPath, func(w http.ResponseWriter, r *http.Request) {
		p.handleWebhook(w, r, "")
	})
	mux.HandleFunc("/mirror/", p.handleMirrorWebhook)
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", handleReadyz)

	server := &http.Server{Addr: p.Listen, Handler: mux}
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		b.OnError(err, nil)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleReadyz(w http.ResponseWriter, r *http.Request) {
	if app.IsShuttingDown() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if err := postgresql.Ping(); e.IsNonNil(err) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if err := rabbitmq.Ping(); e.IsNonNil(err) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (p *MirrorWebhookPoller) handleMirrorWebhook(w http.ResponseWriter, r *http.Request) {
	if !p.authorizeWebhook(w, r) {
		return
	}
	unique := strings.TrimPrefix(r.URL.Path, "/mirror/")
	unique = strings.Trim(unique, "/")
	if unique == "" {
		http.NotFound(w, r)
		return
	}
	mirror, err := models.FindActiveMirrorByUnique(postgresql.GetDB(), unique, time.Now())
	if e.IsNonNil(err) {
		log.Printf("mirror webhook lookup failed unique=%s err=%s", unique, err.JSON())
		http.NotFound(w, r)
		return
	}
	p.handleAuthorizedWebhook(w, r, models.MirrorIDHeaderValue(mirror.ID))
}

// authorizeWebhook rejects requests that don't carry the secret token Telegram echoes
// back for webhooks registered with secret_token. It must run before reading the body.
func (p *MirrorWebhookPoller) authorizeWebhook(w http.ResponseWriter, r *http.Request) bool {
	if p.SecretToken == "" {
		// Startup validation makes the secret mandatory; this is defense in depth.
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	provided := r.Header.Get(sharedTelegram.WebhookSecretHeader)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(p.SecretToken)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

func (p *MirrorWebhookPoller) handleWebhook(w http.ResponseWriter, r *http.Request, mirrorID string) {
	if !p.authorizeWebhook(w, r) {
		return
	}
	p.handleAuthorizedWebhook(w, r, mirrorID)
}

func (p *MirrorWebhookPoller) handleAuthorizedWebhook(w http.ResponseWriter, r *http.Request, mirrorID string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var update tele.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := addUpdate(&update, mirrorID); err != nil {
		// Backpressure / shutdown: tell Telegram to retry later instead of dropping.
		if errors.Is(err, app.ErrShuttingDown) || errors.Is(err, app.ErrQueueFull) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func webhookPath(publicURL string) string {
	parsed, err := url.Parse(publicURL)
	if err != nil || parsed.Path == "" {
		return "/"
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return "/" + parsed.Path
	}
	return parsed.Path
}
