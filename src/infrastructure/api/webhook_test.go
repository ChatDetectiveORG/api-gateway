package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "github.com/ChatDetectiveORG/api-gateway/src/application"
	sharedTelegram "github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"
)

const testSecret = "test-webhook-secret"

func newTestPoller() *MirrorWebhookPoller {
	return &MirrorWebhookPoller{
		Listen:      ":0",
		PublicURL:   "https://bot.example.com/botTOKEN",
		SecretToken: testSecret,
	}
}

func stubAddUpdate(t *testing.T, fn func(update *tele.Update, mirrorID string) error) {
	t.Helper()
	original := addUpdate
	addUpdate = fn
	t.Cleanup(func() { addUpdate = original })
}

func postUpdate(secret string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/botTOKEN", strings.NewReader(`{"update_id":1}`))
	if secret != "" {
		req.Header.Set(sharedTelegram.WebhookSecretHeader, secret)
	}
	return req
}

func TestHandleWebhookRejectsMissingSecret(t *testing.T) {
	stubAddUpdate(t, func(*tele.Update, string) error {
		t.Fatal("update must not be accepted without a secret")
		return nil
	})

	rec := httptest.NewRecorder()
	newTestPoller().handleWebhook(rec, postUpdate(""), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhookRejectsWrongSecret(t *testing.T) {
	stubAddUpdate(t, func(*tele.Update, string) error {
		t.Fatal("update must not be accepted with a wrong secret")
		return nil
	})

	rec := httptest.NewRecorder()
	newTestPoller().handleWebhook(rec, postUpdate("wrong-secret"), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhookRejectsWhenSecretNotConfigured(t *testing.T) {
	stubAddUpdate(t, func(*tele.Update, string) error {
		t.Fatal("update must not be accepted when no secret is configured")
		return nil
	})

	poller := newTestPoller()
	poller.SecretToken = ""
	rec := httptest.NewRecorder()
	poller.handleWebhook(rec, postUpdate(""), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWebhookAcceptsValidSecret(t *testing.T) {
	var accepted bool
	stubAddUpdate(t, func(update *tele.Update, mirrorID string) error {
		accepted = true
		if update.ID != 1 {
			t.Fatalf("unexpected update id %d", update.ID)
		}
		return nil
	})

	rec := httptest.NewRecorder()
	newTestPoller().handleWebhook(rec, postUpdate(testSecret), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !accepted {
		t.Fatal("expected update to be routed")
	}
}

func TestHandleWebhookRejectsNonPost(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/botTOKEN", nil)
	req.Header.Set(sharedTelegram.WebhookSecretHeader, testSecret)
	newTestPoller().handleWebhook(rec, req, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleWebhookRejectsInvalidBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/botTOKEN", strings.NewReader("{not json"))
	req.Header.Set(sharedTelegram.WebhookSecretHeader, testSecret)
	newTestPoller().handleWebhook(rec, req, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleWebhookReturns503OnShutdownAndBackpressure(t *testing.T) {
	for _, backendErr := range []error{app.ErrShuttingDown, app.ErrQueueFull} {
		stubAddUpdate(t, func(*tele.Update, string) error { return backendErr })

		rec := httptest.NewRecorder()
		newTestPoller().handleWebhook(rec, postUpdate(testSecret), "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("error %v: expected 503, got %d", backendErr, rec.Code)
		}
	}
}

func TestMirrorWebhookRejectsMissingSecretBeforeLookup(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mirror/some-unique", strings.NewReader(`{"update_id":1}`))
	newTestPoller().handleMirrorWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWebhookPath(t *testing.T) {
	cases := map[string]string{
		"https://bot.example.com/botTOKEN": "/botTOKEN",
		"https://bot.example.com":          "/",
		"":                                 "/",
	}
	for input, want := range cases {
		if got := webhookPath(input); got != want {
			t.Fatalf("webhookPath(%q) = %q, want %q", input, got, want)
		}
	}
}
