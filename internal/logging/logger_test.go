package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupLogger(t *testing.T) {
	t.Parallel()
	t.Run("json logger writes json", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := SetupLogger(false, LogFormatJSON, &buf)

		logger.Info("hello", "key", "value")

		assert.Contains(t, buf.String(), `"msg":"hello"`)
		assert.Contains(t, buf.String(), `"key":"value"`)
	})

	t.Run("debug enables debug level", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := SetupLogger(true, LogFormatText, &buf)

		logger.Debug("debug message")

		assert.Contains(t, buf.String(), "debug message")
	})

	t.Run("invalid format falls back to json", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := SetupLogger(false, LogFormat("invalid"), &buf)

		logger.LogAttrs(nil, slog.LevelInfo, "fallback")

		assert.Contains(t, buf.String(), `"msg":"fallback"`)
	})
}
