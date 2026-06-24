package flag

import (
	"testing"

	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailConfigsFromDynamicGroup(t *testing.T) {
	t.Parallel()

	t.Run("converts configured email", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.smtp-port=2525",
			"--email.ops.smtp-user=user",
			"--email.ops.smtp-pass=pass",
			"--email.ops.smtp-skip-insecure=true",
			"--email.ops.send-resolved=true",
			"--email.ops.from=overdue@example.test",
			"--email.ops.to=ops@example.test",
			"--email.ops.headers=X-Trace=yes",
			"--email.ops.custom-data=owner=platform",
			"--email.ops.subject-template=subject",
			"--email.ops.resolved-subject-template=resolved subject",
			"--email.ops.title-template=email title",
			"--email.ops.resolved-title-template=email resolved title",
			"--email.ops.text-template=email text",
			"--email.ops.resolved-text-template=email resolved text",
			"--email.ops.template=email.tmpl",
		})

		configs, err := emailConfigsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.NoError(t, err)
		require.Len(t, configs, 1)
		assert.Equal(t, targets.EmailConfig{
			Name:                    "ops",
			Host:                    "smtp.example.test",
			Port:                    2525,
			User:                    "user",
			Pass:                    "pass",
			SkipTLSVerify:           true,
			SendResolved:            true,
			From:                    "overdue@example.test",
			To:                      []string{"ops@example.test"},
			Headers:                 map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"},
			SubjectTemplate:         "subject",
			ResolvedSubjectTemplate: "resolved subject",
			ContentTemplates: render.ContentTemplates{
				Title:         "email title",
				ResolvedTitle: "email resolved title",
				Text:          "email text",
				ResolvedText:  "email resolved text",
				CustomData:    map[string]string{"owner": "platform"},
			},
			Template: "email.tmpl",
		}, configs[0])
	})
}

func TestEmailConfigsFromDynamicGroupDefaults(t *testing.T) {
	t.Parallel()

	fs := notifyTestFlagSet(t, []string{
		"--email.ops.smtp-host=smtp.example.test",
		"--email.ops.from=overdue@example.test",
		"--email.ops.to=ops@example.test",
		"--email.ops.headers=X-Trace=yes",
		"--email.ops.headers=X-Mailer=custom",
	})
	emailGroup := fs.DynamicGroup("email")

	configs, err := emailConfigsFromDynamicGroup("dev", emailGroup)

	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, targets.EmailConfig{
		Name:                    "ops",
		Host:                    "smtp.example.test",
		Port:                    587,
		From:                    "overdue@example.test",
		To:                      []string{"ops@example.test"},
		Headers:                 map[string]string{"X-Mailer": "overdue/dev", "X-Trace": "yes"},
		SubjectTemplate:         "{{ .Title }}",
		ResolvedSubjectTemplate: "{{ .Title }}",
		ContentTemplates:        notifyTestDefaultContentTemplates(),
	}, configs[0])
}

func TestEmailConfigsFromDynamicGroupErrors(t *testing.T) {
	t.Parallel()

	t.Run("returns header parse error", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.headers=invalid",
		})

		_, err := emailConfigsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--email.ops.headers"`)
	})

	t.Run("returns custom data parse error", func(t *testing.T) {
		t.Parallel()

		fs := notifyTestFlagSet(t, []string{
			"--email.ops.smtp-host=smtp.example.test",
			"--email.ops.custom-data=invalid",
		})

		_, err := emailConfigsFromDynamicGroup("dev", fs.DynamicGroup("email"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid "--email.ops.custom-data"`)
	})
}
