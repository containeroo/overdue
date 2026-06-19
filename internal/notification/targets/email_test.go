package targets

import (
	"context"
	"testing"

	"github.com/containeroo/overdue/internal/notification/delivery"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmail(t *testing.T) {
	t.Parallel()
	t.Run("copies recipient list", func(t *testing.T) {
		t.Parallel()

		to := []string{"ops@example.test"}
		email := NewEmail(EmailConfig{To: to}, EmailRenderer{}, testLogger())
		to[0] = "mutated@example.test"

		assert.Equal(t, []string{"ops@example.test"}, email.Config().To)
	})

	t.Run("copies headers", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{"X-Mailer": "overdue/dev"}
		email := NewEmail(EmailConfig{Headers: headers}, EmailRenderer{}, testLogger())
		headers["X-Mailer"] = "mutated"

		assert.Equal(t, map[string]string{"X-Mailer": "overdue/dev"}, email.Config().Headers)
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "email logger must not be nil", func() {
			_ = NewEmail(EmailConfig{}, EmailRenderer{}, nil)
		})
	})
}

func TestEmailConfig(t *testing.T) {
	t.Parallel()
	t.Run("returns defensive copy", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{To: []string{"ops@example.test"}}, EmailRenderer{}, testLogger())
		cfg := email.Config()
		cfg.To[0] = "mutated@example.test"

		assert.Equal(t, []string{"ops@example.test"}, email.Config().To)
	})

	t.Run("returns defensive header copy", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{Headers: map[string]string{"X-Mailer": "overdue/dev"}}, EmailRenderer{}, testLogger())
		cfg := email.Config()
		cfg.Headers["X-Mailer"] = "mutated"

		assert.Equal(t, map[string]string{"X-Mailer": "overdue/dev"}, email.Config().Headers)
	})
}

func TestEmailNotify(t *testing.T) {
	t.Parallel()
	t.Run("skips resolved events when disabled", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{SendResolved: false}, EmailRenderer{}, testLogger())

		err := email.Notify(context.Background(), testResolvedEvent())

		require.ErrorIs(t, err, delivery.ErrSkipped)
	})

	t.Run("returns render errors before smtp delivery", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewEmailRenderer(nil, writeTemplate(t, `{{ .Missing.Field }}`), "", "", render.DefaultContentTemplates())
		require.NoError(t, err)
		email := NewEmail(EmailConfig{
			Host:         "127.0.0.1",
			Port:         1,
			From:         "overdue@example.test",
			To:           []string{"ops@example.test"},
			SendResolved: true,
			Template:     writeTemplate(t, `{{ .Missing.Field }}`),
			ContentTemplates: render.ContentTemplates{
				Title:         "title",
				ResolvedTitle: "resolved title",
				Text:          "text",
				ResolvedText:  "resolved text",
			},
		}, renderer, testLogger())

		err = email.Notify(context.Background(), testEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "render notification template")
	})
}

func TestEmailSend(t *testing.T) {
	t.Parallel()
	t.Run("returns smtp dial errors", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{Host: "127.0.0.1", Port: 1}, EmailRenderer{}, testLogger())

		err := email.send([]string{"ops@example.test"}, []byte("body"))

		require.Error(t, err)
	})
}

func TestAppendEmailHeaders(t *testing.T) {
	t.Parallel()

	lines := appendEmailHeaders([]string{"Subject: test"}, map[string]string{
		"X-Mailer": "overdue/dev",
		"X-Zeta":   "last",
	})

	assert.Equal(t, []string{
		"Subject: test",
		"X-Mailer: overdue/dev",
		"X-Zeta: last",
	}, lines)
}

func TestEmailTlsConfig(t *testing.T) {
	t.Parallel()
	t.Run("uses configured server name and verification mode", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{Host: "smtp.example.test", SkipTLSVerify: true}, EmailRenderer{}, testLogger())

		cfg := email.tlsConfig()

		require.NotNil(t, cfg)
		assert.Equal(t, "smtp.example.test", cfg.ServerName)
		assert.True(t, cfg.InsecureSkipVerify)
	})

	t.Run("keeps tls verification enabled by default", func(t *testing.T) {
		t.Parallel()

		email := NewEmail(EmailConfig{Host: "smtp.example.test"}, EmailRenderer{}, testLogger())

		cfg := email.tlsConfig()

		require.NotNil(t, cfg)
		assert.False(t, cfg.InsecureSkipVerify)
	})
}
