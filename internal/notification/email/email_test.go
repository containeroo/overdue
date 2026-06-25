package email

import (
	"context"
	"testing"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	t.Run("copies recipient list", func(t *testing.T) {
		t.Parallel()

		to := []string{"ops@example.test"}
		email := New(Config{To: to}, Renderer{}, testLogger())
		to[0] = "mutated@example.test"

		assert.Equal(t, []string{"ops@example.test"}, email.Config().To)
	})

	t.Run("copies headers", func(t *testing.T) {
		t.Parallel()

		headers := map[string]string{"X-Mailer": "overdue/dev"}
		email := New(Config{Headers: headers}, Renderer{}, testLogger())
		headers["X-Mailer"] = "mutated"

		assert.Equal(t, map[string]string{"X-Mailer": "overdue/dev"}, email.Config().Headers)
	})

	t.Run("panics without logger", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "email logger must not be nil", func() {
			_ = New(Config{}, Renderer{}, nil)
		})
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()
	t.Run("returns defensive copy", func(t *testing.T) {
		t.Parallel()

		email := New(Config{To: []string{"ops@example.test"}}, Renderer{}, testLogger())
		cfg := email.Config()
		cfg.To[0] = "mutated@example.test"

		assert.Equal(t, []string{"ops@example.test"}, email.Config().To)
	})

	t.Run("returns defensive header copy", func(t *testing.T) {
		t.Parallel()

		email := New(Config{Headers: map[string]string{"X-Mailer": "overdue/dev"}}, Renderer{}, testLogger())
		cfg := email.Config()
		cfg.Headers["X-Mailer"] = "mutated"

		assert.Equal(t, map[string]string{"X-Mailer": "overdue/dev"}, email.Config().Headers)
	})
}

func TestNotifierTarget(t *testing.T) {
	t.Parallel()

	t.Run("returns target metadata", func(t *testing.T) {
		t.Parallel()

		email := New(Config{Name: "primary"}, Renderer{}, testLogger())

		assert.Equal(t, target.Target{Type: "email", Name: "primary"}, email.Target())
	})
}

func TestNotifierNotify(t *testing.T) {
	t.Parallel()
	t.Run("skips resolved events when disabled", func(t *testing.T) {
		t.Parallel()

		email := New(Config{SendResolved: false}, Renderer{}, testLogger())

		err := email.Notify(context.Background(), testResolvedEvent())

		require.ErrorIs(t, err, target.ErrSkipped)
	})

	t.Run("returns render errors before smtp delivery", func(t *testing.T) {
		t.Parallel()

		renderer, err := NewRenderer(nil, writeTemplate(t, `{{ .Missing.Field }}`), "", "", render.DefaultContentTemplates())
		require.NoError(t, err)
		email := New(Config{
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

func TestNotifierSend(t *testing.T) {
	t.Parallel()
	t.Run("returns smtp dial errors", func(t *testing.T) {
		t.Parallel()

		email := New(Config{Host: "127.0.0.1", Port: 1}, Renderer{}, testLogger())

		err := email.send([]string{"ops@example.test"}, []byte("body"))

		require.Error(t, err)
	})
}

func TestAppendNotifierHeaders(t *testing.T) {
	t.Parallel()

	lines := appendNotifierHeaders([]string{"Subject: test"}, map[string]string{
		"X-Mailer": "overdue/dev",
		"X-Zeta":   "last",
	})

	assert.Equal(t, []string{
		"Subject: test",
		"X-Mailer: overdue/dev",
		"X-Zeta: last",
	}, lines)
}

func TestNotifierTlsConfig(t *testing.T) {
	t.Parallel()
	t.Run("uses configured server name and verification mode", func(t *testing.T) {
		t.Parallel()

		email := New(Config{Host: "smtp.example.test", SkipTLSVerify: true}, Renderer{}, testLogger())

		cfg := email.tlsConfig()

		require.NotNil(t, cfg)
		assert.Equal(t, "smtp.example.test", cfg.ServerName)
		assert.True(t, cfg.InsecureSkipVerify)
	})

	t.Run("keeps tls verification enabled by default", func(t *testing.T) {
		t.Parallel()

		email := New(Config{Host: "smtp.example.test"}, Renderer{}, testLogger())

		cfg := email.tlsConfig()

		require.NotNil(t, cfg)
		assert.False(t, cfg.InsecureSkipVerify)
	})
}
