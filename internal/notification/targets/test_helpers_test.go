package targets

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/stretchr/testify/require"
)

// errReader always fails reads.
type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

// writeTemplate writes a temporary notification template.
func writeTemplate(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "template.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// testTemplateFS returns built-in template fixtures.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"email-html.tmpl": {
			Data: []byte(`<h1>{{ .Title }}</h1><p>{{ .Text }}</p>`),
		},
		"webhook.tmpl": {
			Data: []byte(`{"title":{{ json .Title }},"text":{{ json .Text }}}`),
		},
	}
}

// testLogger returns a discard logger.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testContentTemplates returns deterministic notification content templates.
func testContentTemplates() render.ContentTemplates {
	return render.ContentTemplates{
		Title:         `alerting {{ .CheckInName }}`,
		ResolvedTitle: `resolved {{ .CheckInName }}`,
		Text:          `{{ .CheckInName }} is down`,
		ResolvedText:  `{{ .CheckInName }} is up`,
	}
}

// testEvent returns a complete monitor event for target tests.
func testEvent() monitor.Event {
	lastCheckIn := time.Date(2026, 6, 19, 14, 28, 22, 0, time.UTC)
	expectedBy := lastCheckIn.Add(5 * time.Second)
	alertingAt := expectedBy.Add(3 * time.Second)
	return monitor.Event{
		IncidentID:     "incident-1",
		NotificationID: "notification-1",
		CheckInName:    "prometheus",
		LastCheckIn:    lastCheckIn,
		ExpectedBy:     expectedBy,
		OverdueSince:   expectedBy,
		AlertingAt:     alertingAt,
		Now:            alertingAt,
		Phase:          monitor.PhaseAlerting,
		Status:         monitor.StatusAlerting,
	}
}

// testResolvedEvent returns a complete resolved monitor event for target tests.
func testResolvedEvent() monitor.Event {
	event := testEvent()
	event.NotificationID = "notification-2"
	event.Now = event.AlertingAt.Add(10 * time.Second)
	event.Phase = monitor.PhaseAwaiting
	event.Status = monitor.StatusResolved
	event.Resolved = true
	return event
}
