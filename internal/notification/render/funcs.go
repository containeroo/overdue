package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"
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
	return template.FuncMap{
		"ago":        agoTemplateValue,
		"default":    defaultTemplateValue,
		"duration":   durationTemplateValue,
		"json":       jsonTemplateValue,
		"lower":      lowerTemplateValue,
		"trim":       trimTemplateValue,
		"upper":      upperTemplateValue,
		"when":       conditionalString,
		"withPrefix": withPrefix,
		"withSuffix": withSuffix,
	}
}

// conditionalString returns trueValue when condition is true and falseValue otherwise.
//
// The argument order favors natural inline conditional usage:
//
//	{{ when .Resolved "Resolved at" "Notified at" }}
func conditionalString(condition bool, trueValue, falseValue string) string {
	if condition {
		return trueValue
	}
	return falseValue
}

// defaultTemplateValue returns fallback when value is empty or the zero value.
//
// The argument order matches common template usage:
//
//	{{ default "fallback" .Value }}
//	{{ .Value | default "fallback" }}
func defaultTemplateValue(fallback, value any) any {
	if isZeroTemplateValue(value) {
		return fallback
	}
	return value
}

// isZeroTemplateValue reports whether value should use a template fallback.
func isZeroTemplateValue(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return true
		}
	}

	return v.IsZero()
}

// trimTemplateValue returns value as a string with surrounding whitespace removed.
func trimTemplateValue(value any) string {
	return strings.TrimSpace(templateString(value))
}

// upperTemplateValue returns value as an uppercase string.
func upperTemplateValue(value any) string {
	return strings.ToUpper(templateString(value))
}

// lowerTemplateValue returns value as a lowercase string.
func lowerTemplateValue(value any) string {
	return strings.ToLower(templateString(value))
}

// templateString converts a template value to a string.
func templateString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
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

// durationTemplateValue renders a duration value as a Go duration string.
func durationTemplateValue(value any) (string, error) {
	d, err := templateDuration(value)
	if err != nil {
		return "", err
	}
	return d.String(), nil
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

// templateDuration converts a template value to time.Duration.
func templateDuration(value any) (time.Duration, error) {
	switch v := value.(type) {
	case time.Duration:
		return v, nil
	case *time.Duration:
		if v == nil {
			return 0, fmt.Errorf("duration value must not be nil")
		}
		return *v, nil
	case string:
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("parse duration %q: %w", v, err)
		}
		return d, nil
	default:
		return 0, fmt.Errorf("duration value must be time.Duration or duration string, got %T", value)
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

// jsonTemplateValue renders any template value as a JSON literal.
func jsonTemplateValue(value any) (literal string, err error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// withPrefix returns value with prefix prepended when it is not already present.
//
// The argument order supports both direct and pipeline usage:
//
//	{{ withPrefix "#" .CustomData.channel }}
//	{{ .CustomData.channel | withPrefix "#" }}
func withPrefix(prefix string, value any) string {
	if value == nil {
		return ""
	}

	text := strings.TrimSpace(fmt.Sprint(value))
	prefix = strings.TrimSpace(prefix)

	if text == "" || prefix == "" || strings.HasPrefix(text, prefix) {
		return text
	}

	return prefix + text
}

// withSuffix returns value with suffix appended when it is not already present.
//
// The argument order supports both direct and pipeline usage:
//
//	{{ withSuffix "/" .CustomData.path }}
//	{{ .CustomData.path | withSuffix "/" }}
func withSuffix(suffix string, value any) string {
	if value == nil {
		return ""
	}

	text := strings.TrimSpace(fmt.Sprint(value))
	suffix = strings.TrimSpace(suffix)

	if text == "" || suffix == "" || strings.HasSuffix(text, suffix) {
		return text
	}

	return text + suffix
}
