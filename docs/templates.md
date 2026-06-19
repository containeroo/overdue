# Templates

Overdue supports built-in and custom Go templates for notification payloads.

Overdue validates configured templates at startup by rendering both an alerting and a resolved sample event. Invalid templates fail fast before the server starts.

Webhook templates must render valid JSON.

Email templates render text or HTML email body content.

## Built-in templates

Available built-in templates:

| Name                              | Use                                  |
| --------------------------------- | ------------------------------------ |
| `builtin:email-html`              | HTML email body.                     |
| `builtin:slack-incoming-webhook`  | Slack incoming webhook JSON payload. |
| `builtin:slack-chat-post-message` | Slack Web API JSON payload.          |

Use a built-in template:

```sh
--webhook.ops.template=builtin:slack-incoming-webhook
--email.primary.template=builtin:email-html
```

Use a custom template file:

```sh
--webhook.ops.template=/etc/overdue/slack.tmpl
--email.primary.template=/etc/overdue/email.html.tmpl
```

## Template data

Notification templates receive a check-in lifecycle event.

| Field             | Type        | Description                                                                          |
| ----------------- | ----------- | ------------------------------------------------------------------------------------ |
| `.IncidentID`     | string      | Stable ID shared by the alerting and resolved notification for one overdue incident. |
| `.NotificationID` | string      | Stable ID for one notification message. Reused across delivery retries.              |
| `.CheckInName`    | string      | Configured check-in monitor name.                                                    |
| `.LastCheckIn`    | `time.Time` | Last received check-in timestamp.                                                    |
| `.ExpectedBy`     | `time.Time` | Time when the next check-in was expected.                                            |
| `.OverdueSince`   | `time.Time` | Time when the monitor became overdue.                                                |
| `.AlertingAt`     | `time.Time` | Time when alerting started and notifications were created.                           |
| `.Now`            | `time.Time` | Time of the notification event.                                                      |
| `.Phase`          | string      | Monitor phase, for example `alerting` or `awaiting`.                                 |
| `.Status`         | string      | Notification event status: `alerting` or `resolved`.                                 |
| `.Resolved`       | bool        | `true` for resolved notifications.                                                   |
| `.Title`          | string      | Rendered notification title. Available in body and subject templates.                |
| `.Text`           | string      | Rendered notification text. Available in body templates.                             |

Example:

```gotemplate
{{ .Title }}

Check-in: {{ .CheckInName }}
Status: {{ .Status }}
Expected by: {{ .ExpectedBy.Format "2006-01-02 15:04:05 MST" }}
Alerting at: {{ .AlertingAt.Format "2006-01-02 15:04:05 MST" }}
```

## Template functions

Overdue templates use Go `text/template` and include these helper functions:

| Function   | Description                                  | Example                                            |
| ---------- | -------------------------------------------- | -------------------------------------------------- |
| `json`     | Render a value as a JSON literal.            | `{{ json .Text }}`                                 |
| `when`     | Inline conditional string selection.         | `{{ when .Resolved "Resolved at" "Notified at" }}` |
| `default`  | Return fallback when value is empty or zero. | `{{ default "unknown" .CheckInName }}`             |
| `trim`     | Trim surrounding whitespace.                 | `{{ trim .CheckInName }}`                          |
| `upper`    | Convert value to uppercase.                  | `{{ upper .Status }}`                              |
| `lower`    | Convert value to lowercase.                  | `{{ lower .Status }}`                              |
| `ago`      | Render relative time from now.               | `{{ ago .LastCheckIn }}`                           |
| `duration` | Render a duration value.                     | `{{ duration (.AlertingAt.Sub .ExpectedBy) }}`     |

Use Go template `if` for blocks:

```gotemplate
{{ if .Resolved }}
Resolved
{{ else }}
Overdue
{{ end }}
```

Use `when` for inline string choices:

```gotemplate
{{ when .Resolved "Resolved at" "Notified at" }}
```

For JSON webhook templates, wrap dynamic strings with `json`:

```gotemplate
{
  "text": {{ json .Text }},
  "status": {{ json .Status }}
}
```
