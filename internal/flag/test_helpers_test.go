package flag

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containeroo/overdue/internal/notification/email"
	"github.com/containeroo/overdue/internal/notification/webhook"
	"github.com/stretchr/testify/require"
)

// writeTemplate writes a temporary template file.
func writeTemplate(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "template.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// requireWebhookConfig returns a named webhook config.
func requireWebhookConfig(t *testing.T, configs []webhook.Config, name string) webhook.Config {
	t.Helper()
	for _, cfg := range configs {
		if cfg.Name == name {
			return cfg
		}
	}
	require.FailNowf(t, "missing webhook config", "webhook %q not found", name)
	return webhook.Config{}
}

// requireEmailConfig returns a named email config.
func requireEmailConfig(t *testing.T, configs []email.Config, name string) email.Config {
	t.Helper()
	for _, cfg := range configs {
		if cfg.Name == name {
			return cfg
		}
	}
	require.FailNowf(t, "missing email config", "email %q not found", name)
	return email.Config{}
}
