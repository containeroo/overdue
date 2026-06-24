package render

import (
	"fmt"
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
			`{{ .Channel | default "#alertmanager" | withPrefix "#" }} {{ when .Resolved "up" "down" }}`,
		)
		require.NoError(t, err)

		got, err := ExecuteInlineTemplate(tmpl, map[string]any{
			"Channel":  "alerts",
			"Resolved": true,
		})

		require.NoError(t, err)
		assert.Equal(t, "#alerts up", got)
	})
}

func TestConditionalString(t *testing.T) {
	t.Parallel()

	t.Run("selects true value", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "yes", conditionalString(true, "yes", "no"))
	})

	t.Run("selects false value", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "no", conditionalString(false, "yes", "no"))
	})
}

func TestDefaultTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("returns fallback for zero values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "fallback", defaultTemplateValue("fallback", ""))
		assert.Equal(t, "fallback", defaultTemplateValue("fallback", 0))
		assert.Equal(t, "fallback", defaultTemplateValue("fallback", false))
		assert.Equal(t, "fallback", defaultTemplateValue("fallback", []string(nil)))
	})

	t.Run("returns value for non zero values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "value", defaultTemplateValue("fallback", "value"))
		assert.Equal(t, 42, defaultTemplateValue("fallback", 42))
		assert.Equal(t, true, defaultTemplateValue("fallback", true))
	})
}

func TestIsZeroTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("detects zero values", func(t *testing.T) {
		t.Parallel()

		var ptr *string

		assert.True(t, isZeroTemplateValue(nil))
		assert.True(t, isZeroTemplateValue(ptr))
		assert.True(t, isZeroTemplateValue(""))
		assert.True(t, isZeroTemplateValue(time.Time{}))
	})

	t.Run("detects non zero values", func(t *testing.T) {
		t.Parallel()

		value := "value"

		assert.False(t, isZeroTemplateValue(&value))
		assert.False(t, isZeroTemplateValue("value"))
		assert.False(t, isZeroTemplateValue(time.Date(2026, 6, 19, 14, 28, 22, 0, time.UTC)))
	})
}

func TestTrimTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "value", trimTemplateValue(" value\n"))
	})
}

func TestUpperTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("uppercases values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "PROMETHEUS", upperTemplateValue("prometheus"))
	})
}

func TestLowerTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("lowercases values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "prometheus", lowerTemplateValue("PROMETHEUS"))
	})
}

func TestTemplateString(t *testing.T) {
	t.Parallel()

	t.Run("converts nil to empty string", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, templateString(nil))
	})

	t.Run("uses fmt string conversion", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "42", templateString(42))
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

func TestDurationTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("renders duration values", func(t *testing.T) {
		t.Parallel()

		duration := 90 * time.Second

		got, err := durationTemplateValue(&duration)

		require.NoError(t, err)
		assert.Equal(t, "1m30s", got)
	})

	t.Run("renders duration strings", func(t *testing.T) {
		t.Parallel()

		got, err := durationTemplateValue("2m")

		require.NoError(t, err)
		assert.Equal(t, "2m0s", got)
	})

	t.Run("returns conversion errors", func(t *testing.T) {
		t.Parallel()

		_, err := durationTemplateValue("not a duration")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse duration")
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

func TestTemplateDuration(t *testing.T) {
	t.Parallel()

	t.Run("accepts duration values", func(t *testing.T) {
		t.Parallel()

		want := 90 * time.Second

		got, err := templateDuration(want)
		require.NoError(t, err)
		assert.Equal(t, want, got)

		got, err = templateDuration(&want)
		require.NoError(t, err)
		assert.Equal(t, want, got)

		got, err = templateDuration("90s")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		t.Parallel()

		var nilDuration *time.Duration

		_, err := templateDuration(nilDuration)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be nil")

		_, err = templateDuration("not a duration")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse duration")

		_, err = templateDuration(42)
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

func TestJsonTemplateValue(t *testing.T) {
	t.Parallel()

	t.Run("renders json literal", func(t *testing.T) {
		t.Parallel()

		got, err := jsonTemplateValue(map[string]any{"name": "prometheus", "count": float64(2)})

		require.NoError(t, err)
		assert.JSONEq(t, `{"count":2,"name":"prometheus"}`, got)
	})

	t.Run("returns marshal errors", func(t *testing.T) {
		t.Parallel()

		_, err := jsonTemplateValue(func() {})

		require.Error(t, err)
		assert.Contains(t, fmt.Sprint(err), "unsupported type")
	})
}

func TestWithPrefix(t *testing.T) {
	t.Parallel()

	t.Run("adds missing prefix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "#alerts", withPrefix("#", "alerts"))
	})

	t.Run("keeps existing prefix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "#alerts", withPrefix("#", "#alerts"))
	})

	t.Run("trims values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "#alerts", withPrefix(" # ", " alerts "))
	})

	t.Run("returns empty for nil value", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, withPrefix("#", nil))
	})

	t.Run("returns value unchanged for empty prefix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "alerts", withPrefix("", "alerts"))
	})
}

func TestWithSuffix(t *testing.T) {
	t.Parallel()

	t.Run("adds missing suffix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "path/", withSuffix("/", "path"))
	})

	t.Run("keeps existing suffix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "path/", withSuffix("/", "path/"))
	})

	t.Run("trims values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "path/", withSuffix(" / ", " path "))
	})

	t.Run("returns empty for nil value", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, withSuffix("/", nil))
	})

	t.Run("returns value unchanged for empty suffix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "path", withSuffix("", "path"))
	})
}
