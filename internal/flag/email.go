package flag

import (
	"errors"

	"github.com/containeroo/httputils"
	"github.com/containeroo/overdue/internal/notification/targets"
	"github.com/containeroo/tinyflags"
)

// registerEmailFlags registers dynamic email notification flags.
func registerEmailFlags(tf *tinyflags.FlagSet) {
	email := tf.DynamicGroup("email").Title("Email")
	email.String("smtp-host", "", "SMTP host").
		Required().
		Placeholder("HOST")
	email.Int("smtp-port", 587, "SMTP port").
		Validate(func(port int) error {
			if port < 0 {
				return errors.New("smtp-port must be a positive integer")
			}
			return nil
		}).
		Placeholder("PORT")
	email.String("smtp-user", "", "SMTP username")
	email.String("smtp-pass", "", "SMTP password").
		Requires("smtp-user").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)
	email.Bool("smtp-skip-insecure", false, "Skip SMTP TLS certificate verification")
	email.Bool("send-resolved", false, "Send a resolved email notification when check-ins resume after alerting")
	email.String("subject-template", "{{ .Title }}", "Template for overdue email subject")
	email.String("resolved-subject-template", "{{ .Title }}", "Template for resolved email subject")
	email.String("title-template", `[OVERDUE] Event Notification`, "Template for overdue email body title")
	email.String("resolved-title-template", `[RESOLVED] [OVERDUE] Event Notification`, "Template for resolved email body title")
	email.String("text-template", `Check-in "{{ .CheckInName }}" is overdue:`, "Template for overdue email body text")
	email.String("resolved-text-template", `Check-in "{{ .CheckInName }}" is resolved:`, "Template for resolved email body text")
	email.String("from", "", "Email sender address").
		Required().
		Placeholder("ADDR")
	email.StringSlice("to", []string{}, "Email recipient address").
		Required().
		Placeholder("ADDR")
	email.StringSlice("headers", nil, "Email headers in KEY=VALUE format").
		Validate(func(raw string) error {
			_, err := httputils.ParseHeaders(raw, false)
			return err
		}).
		Placeholder("KEY=VALUE")
	email.StringSlice("custom-data", nil, "Custom email template data in KEY=VALUE format").
		Validate(validateKeyValue).
		Placeholder("KEY=VALUE")
	email.String("template", "", "Path or builtin:<name> template for email body").
		Required().
		Placeholder("PATH|builtin:NAME")
}

// emailConfigsFromDynamicGroup converts one parsed email dynamic group into typed config.
func emailConfigsFromDynamicGroup(version string, group *tinyflags.DynamicGroup) ([]targets.EmailConfig, error) {
	ids := sortedInstances(group)
	configs := make([]targets.EmailConfig, 0, len(ids))

	for _, id := range ids {
		headers, err := webhookHeadersMap(group.Name(), id, tinyflags.GetOrDefaultDynamic[[]string](group, id, "headers"))
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = make(map[string]string, 1)
		}
		headers["X-Mailer"] = "overdue/" + version

		customData, err := keyValueMap(group.Name(), id, "custom-data", tinyflags.GetOrDefaultDynamic[[]string](group, id, "custom-data"))
		if err != nil {
			return nil, err
		}

		content := contentTemplates(group, id)
		content.CustomData = customData

		configs = append(configs, targets.EmailConfig{
			Name:                    id,
			Host:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-host"),
			Port:                    tinyflags.GetOrDefaultDynamic[int](group, id, "smtp-port"),
			User:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-user"),
			Pass:                    tinyflags.GetOrDefaultDynamic[string](group, id, "smtp-pass"),
			SkipTLSVerify:           tinyflags.GetOrDefaultDynamic[bool](group, id, "smtp-skip-insecure"),
			SendResolved:            tinyflags.GetOrDefaultDynamic[bool](group, id, "send-resolved"),
			From:                    tinyflags.GetOrDefaultDynamic[string](group, id, "from"),
			To:                      tinyflags.GetOrDefaultDynamic[[]string](group, id, "to"),
			Headers:                 headers,
			SubjectTemplate:         tinyflags.GetOrDefaultDynamic[string](group, id, "subject-template"),
			ResolvedSubjectTemplate: tinyflags.GetOrDefaultDynamic[string](group, id, "resolved-subject-template"),
			ContentTemplates:        content,
			Template:                tinyflags.GetOrDefaultDynamic[string](group, id, "template"),
		})
	}

	return configs, nil
}
