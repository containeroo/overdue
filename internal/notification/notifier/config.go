package notifier

import (
	"github.com/containeroo/overdue/internal/notification/render"
	"github.com/containeroo/overdue/internal/notification/targets"
)

// Config contains all configured notification targets.
type Config struct {
	App      render.AppData
	Webhooks []targets.WebhookConfig
	Emails   []targets.EmailConfig
}
