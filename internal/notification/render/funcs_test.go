package render

import (
	"testing"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInlineTemplate(t *testing.T) {
	t.Parallel()

	t.Run("parses helpers", func(t *testing.T) {
		t.Parallel()

		tmpl, err := ParseInlineTemplate("subject", `{{ upper (trim .CheckInName) }} {{ when .Resolved "up" "down" }}`)
		require.NoError(t, err)

		got, err := ExecuteInlineTemplate(tmpl, monitor.Event{CheckInName: " prometheus "})
		require.NoError(t, err)
		assert.Equal(t, "PROMETHEUS down", got)
	})

	t.Run("wraps parse errors with template name", func(t *testing.T) {
		t.Parallel()

		_, err := ParseInlineTemplate("subject", `{{ if }}`)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse subject")
	})
}

func TestExecuteInlineTemplate(t *testing.T) {
	t.Parallel()

	t.Run("executes template against event", func(t *testing.T) {
		t.Parallel()

		tmpl, err := ParseInlineTemplate("subject", `{{ .CheckInName }} {{ .Status }}`)
		require.NoError(t, err)

		got, err := ExecuteInlineTemplate(tmpl, monitor.Event{CheckInName: "prometheus", Status: monitor.StatusAlerting})

		require.NoError(t, err)
		assert.Equal(t, "prometheus alerting", got)
	})

	t.Run("returns execution errors", func(t *testing.T) {
		t.Parallel()

		tmpl, err := ParseInlineTemplate("subject", `{{ .Missing.Field }}`)
		require.NoError(t, err)

		_, err = ExecuteInlineTemplate(tmpl, monitor.Event{})

		require.Error(t, err)
	})
}

func TestTemplateFuncs(t *testing.T) {
	t.Parallel()

	t.Run("contains all public template helpers", func(t *testing.T) {
		t.Parallel()

		funcs := templateFuncs()

		for _, name := range []string{
			"ago",
			"default",
			"duration",
			"json",
			"lower",
			"optional",
			"trim",
			"upper",
			"when",
			"withPrefix",
			"withSuffix",
		} {
			assert.Contains(t, funcs, name)
		}
	})
}

func TestTemplatePipelines(t *testing.T) {
	t.Parallel()

	t.Run("supports pipeline-friendly helper order", func(t *testing.T) {
		t.Parallel()

		tmpl, err := ParseInlineTemplate(
			"pipeline",
			`{{ .Channel | default "#alertmanager" | withPrefix "#" }} {{ when .Resolved "up" "down" }}{{ .StatusURL | optional " %s" }}`,
		)
		require.NoError(t, err)

		got, err := ExecuteInlineTemplate(tmpl, map[string]any{
			"Channel":   "alerts",
			"Resolved":  true,
			"StatusURL": "https://overdue.example.test/status",
		})

		require.NoError(t, err)
		assert.Equal(t, "#alerts up https://overdue.example.test/status", got)
	})
}

func TestAgoTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("renders past time", func(t *testing.T) {
		t.Parallel()

		got, err := agoTemplateValue(time.Now().Add(-1500 * time.Millisecond))

		require.NoError(t, err)
		assert.Contains(t, got, "ago")
	})

	t.Run("renders future time", func(t *testing.T) {
		t.Parallel()

		got, err := agoTemplateValue(time.Now().Add(1500 * time.Millisecond).Format(time.RFC3339Nano))

		require.NoError(t, err)
		assert.Contains(t, got, "in ")
	})

	t.Run("returns conversion errors", func(t *testing.T) {
		t.Parallel()

		_, err := agoTemplateValue("not a time")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse time")
	})
}

func TestTemplateTime(t *testing.T) {
	t.Parallel()

	t.Run("accepts time values", func(t *testing.T) {
		t.Parallel()

		want := time.Date(2026, 6, 19, 14, 28, 22, 123, time.UTC)

		got, err := templateTime(want)
		require.NoError(t, err)
		assert.True(t, got.Equal(want))

		got, err = templateTime(&want)
		require.NoError(t, err)
		assert.True(t, got.Equal(want))

		got, err = templateTime(want.Format(time.RFC3339Nano))
		require.NoError(t, err)
		assert.True(t, got.Equal(want))
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		t.Parallel()

		var nilTime *time.Time

		_, err := templateTime(nilTime)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be nil")

		_, err = templateTime("not a time")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse time")

		_, err = templateTime(42)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "got int")
	})
}

func TestFormatApproxDuration(t *testing.T) {
	t.Parallel()

	t.Run("formats useful duration precision", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "2s", formatApproxDuration(1500*time.Millisecond))
		assert.Equal(t, "2ms", formatApproxDuration(1500*time.Microsecond))
		assert.Equal(t, "2µs", formatApproxDuration(1500*time.Nanosecond))
		assert.Equal(t, "500ns", formatApproxDuration(500*time.Nanosecond))
		assert.Equal(t, "-2s", formatApproxDuration(-1500*time.Millisecond))
	})
}
