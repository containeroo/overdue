package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/containeroo/tmplfuncs"
)

// ParseInlineTemplate parses a single inline template value.
func ParseInlineTemplate(name, value string) (tmpl *template.Template, err error) {
	tmpl, err = template.New(name).Funcs(templateFuncs()).Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return tmpl, nil
}

// ExecuteInlineTemplate renders an inline template.
func ExecuteInlineTemplate(tmpl *template.Template, data any) (text string, err error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// templateFuncs returns template helper functions.
func templateFuncs() template.FuncMap {
	funcs := tmplfuncs.FuncMap()
	funcs["ago"] = agoTemplateValue

	return funcs
}

// agoTemplateValue renders the distance between value and now.
func agoTemplateValue(value any) (string, error) {
	t, err := templateTime(value)
	if err != nil {
		return "", err
	}

	d := time.Since(t)
	if d < 0 {
		return "in " + formatApproxDuration(-d), nil
	}
	return formatApproxDuration(d) + " ago", nil
}

// templateTime converts a template value to time.Time.
func templateTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case *time.Time:
		if v == nil {
			return time.Time{}, fmt.Errorf("time value must not be nil")
		}
		return *v, nil
	case string:
		t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v))
		if err != nil {
			return time.Time{}, fmt.Errorf("parse time %q: %w", v, err)
		}
		return t, nil
	default:
		return time.Time{}, fmt.Errorf("time value must be time.Time or RFC3339 string, got %T", value)
	}
}

// formatApproxDuration renders duration with useful precision for relative times.
func formatApproxDuration(d time.Duration) string {
	if d < 0 {
		return "-" + formatApproxDuration(-d)
	}

	switch {
	case d >= time.Second:
		return d.Round(time.Second).String()
	case d >= time.Millisecond:
		return d.Round(time.Millisecond).String()
	case d >= time.Microsecond:
		return d.Round(time.Microsecond).String()
	default:
		return d.String()
	}
}
