package templates

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBuiltin(t *testing.T) {
	t.Parallel()
	t.Run("accepts builtin prefix", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsBuiltin("builtin:email-html"))
	})

	t.Run("rejects file path", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsBuiltin("templates/email-html.tmpl"))
	})
}

func TestName(t *testing.T) {
	t.Parallel()
	t.Run("trims prefix and spaces", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "email-html", Name(" builtin:email-html "))
	})
}

func TestTemplates_Read(t *testing.T) {
	t.Parallel()
	t.Run("reads embedded template", func(t *testing.T) {
		t.Parallel()

		filename, body, err := testTemplates().Read("email-html")

		require.NoError(t, err)
		assert.Equal(t, "email-html.tmpl", filename)
		assert.Contains(t, body, "{{ .Title }}")
	})

	t.Run("reads embedded template from builtin source", func(t *testing.T) {
		t.Parallel()

		filename, body, err := testTemplates().Read("builtin:slack-incoming-webhook")

		require.NoError(t, err)
		assert.Equal(t, "slack-incoming-webhook.tmpl", filename)
		assert.Contains(t, body, `"attachments"`)
	})

	t.Run("rejects missing template", func(t *testing.T) {
		t.Parallel()

		_, _, err := testTemplates().Read("missing")

		require.Error(t, err)
		assert.Contains(t, err.Error(), `read built-in template "missing"`)
	})

	t.Run("rejects path separators", func(t *testing.T) {
		t.Parallel()

		_, _, err := testTemplates().Read("builtin:../email-html")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not contain path separators")
	})
}

func TestTemplates_Exists(t *testing.T) {
	t.Parallel()
	t.Run("accepts existing template", func(t *testing.T) {
		t.Parallel()

		err := testTemplates().Exists("builtin:email-html")

		require.NoError(t, err)
	})

	t.Run("rejects missing template", func(t *testing.T) {
		t.Parallel()

		err := testTemplates().Exists("builtin:missing")

		require.Error(t, err)
	})
}

func TestTemplates_Names(t *testing.T) {
	t.Parallel()
	t.Run("returns embedded template names", func(t *testing.T) {
		t.Parallel()

		names := testTemplates().Names()

		assert.Equal(t, []string{
			"email-html",
			"slack-chat-post-message",
			"slack-incoming-webhook",
		}, names)
	})
}

// testTemplates returns a template fixture.
func testTemplates() Templates {
	return New(fstest.MapFS{
		"email-html.tmpl": {
			Data: []byte(`{{ .Title }}`),
		},
		"slack-incoming-webhook.tmpl": {
			Data: []byte(`{"attachments":[]}`),
		},
		"slack-chat-post-message.tmpl": {
			Data: []byte(`{"channel":"#alertmanager","text":{{ json .Text }}}`),
		},
	})
}
