package flag

import (
	"github.com/containeroo/overdue/internal/utils"
	"github.com/containeroo/tinyflags"
)

// registerEmailFlags registers dynamic email notification flags.
func registerEmailFlags(tf *tinyflags.FlagSet) {
	emailGroup := tf.DynamicGroup("email").Title("Email")

	emailGroup.String("smtp-host", "", "SMTP host").
		Required().
		Placeholder("HOST")

	emailGroup.Int("smtp-port", 587, "SMTP port").
		Validate(intAtLeast(1)).
		Placeholder("PORT")

	emailGroup.String("smtp-user", "", "SMTP username")

	emailGroup.String("smtp-pass", "", "SMTP password").
		Requires("smtp-user").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)

	emailGroup.Bool("smtp-tls-skip-verify", false, "Skip SMTP TLS certificate verification")
	emailGroup.Bool("send-resolved", false, "Send a resolved email notification when check-ins resume after alerting")

	emailGroup.String("title-template", defaultTitleTemplate, "Optional template for notification title")

	emailGroup.String("from", "", "Email sender address").
		Required().
		Placeholder("ADDR")

	emailGroup.StringSlice("to", []string{}, "Email recipient address").
		Required().
		Placeholder("ADDR")

	emailGroup.StringSlice("headers", nil, "Email headers in KEY=VALUE format").
		Validate(validateHeader).
		Placeholder("KEY=VALUE").
		OverriddenValueMaskFn(tinyflags.MaskFirstLast)

	emailGroup.StringSlice("custom-data", nil, "Custom email template data in KEY=VALUE format").
		Validate(utils.ValidateKeyValue).
		Placeholder("KEY=VALUE")

	emailGroup.String("template", "", "Path or builtin:<name> template for email body").
		Required().
		Placeholder("PATH|builtin:NAME")
}
