package notifier

import "github.com/containeroo/overdue/internal/notification/targets"

// Config contains all configured notification targets.
type Config struct {
	Webhooks []targets.WebhookConfig
	Emails   []targets.EmailConfig
}
