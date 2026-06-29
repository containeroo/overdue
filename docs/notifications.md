# Notifications

Overdue supports multiple named webhook and email notification targets.

The target name is part of the flag path:

```text
--webhook.<name>.url
--email.<name>.smtp-host
```

For example, this creates a webhook target named `ops`:

```sh
--webhook.ops.url=https://example.com/webhook
```

And this creates an email target named `primary`:

```sh
--email.primary.smtp-host=smtp.example.com
```

If no notification targets are configured, Overdue still runs and records status, but sends no notifications.

Set `--public-url` when notification templates should include externally reachable Overdue links. The value is exposed as `.App.PublicURL`; `.App.CheckInURL` and `.App.StatusURL` are derived from it. `.App.Version` is always available.

## Environment variables

Dynamic notification flags include the target name.

Examples:

```sh
export OVERDUE__WEBHOOK_OPS_URL=https://example.com/webhook
export OVERDUE__WEBHOOK_OPS_METHOD=POST
export OVERDUE__WEBHOOK_OPS_CUSTOM_DATA=channel=#ops
export OVERDUE__WEBHOOK_OPS_TEMPLATE=builtin:slack-incoming-webhook
export OVERDUE__WEBHOOK_OPS_SEND_RESOLVED=true

export OVERDUE__EMAIL_PRIMARY_SMTP_HOST=smtp.example.com
export OVERDUE__EMAIL_PRIMARY_SMTP_USER=user
export OVERDUE__EMAIL_PRIMARY_FROM=overdue@example.com
export OVERDUE__EMAIL_PRIMARY_TO=ops@example.com
export OVERDUE__EMAIL_PRIMARY_TEMPLATE=builtin:email-html
```

## Webhook notifications

Webhook notifications send a JSON request to the configured URL. The default method is `POST`; `PUT`, `PATCH`, and `DELETE` are also supported.

Example:

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --webhook.ops.url=https://example.com/webhook \
  --webhook.ops.method=POST \
  --webhook.ops.custom-data=channel=#ops \
  --webhook.ops.template=builtin:slack-incoming-webhook
```

With environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__WEBHOOK_OPS_URL=https://example.com/webhook
export OVERDUE__WEBHOOK_OPS_METHOD=POST
export OVERDUE__WEBHOOK_OPS_CUSTOM_DATA=channel=#ops
export OVERDUE__WEBHOOK_OPS_TEMPLATE=builtin:slack-incoming-webhook
```

### Webhook flags

| Flag                                       | Environment variable pattern                      | Default                                      | Description                                                                          |
| ------------------------------------------ | ------------------------------------------------- | -------------------------------------------- | ------------------------------------------------------------------------------------ |
| `--webhook.<name>.url`                     | `OVERDUE__WEBHOOK_<NAME>_URL`                     | required                                     | Webhook URL.                                                                         |
| `--webhook.<name>.method`                  | `OVERDUE__WEBHOOK_<NAME>_METHOD`                  | `POST`                                       | HTTP method: `POST`, `PUT`, `PATCH`, or `DELETE`.                                    |
| `--webhook.<name>.timeout`                 | `OVERDUE__WEBHOOK_<NAME>_TIMEOUT`                 | `10s`                                        | HTTP timeout.                                                                        |
| `--webhook.<name>.skip-insecure`           | `OVERDUE__WEBHOOK_<NAME>_SKIP_INSECURE`           | `false`                                      | Skip TLS certificate verification.                                                   |
| `--webhook.<name>.send-resolved`           | `OVERDUE__WEBHOOK_<NAME>_SEND_RESOLVED`           | `false`                                      | Send a resolved notification after check-ins resume.                                 |
| `--webhook.<name>.title-template`          | `OVERDUE__WEBHOOK_<NAME>_TITLE_TEMPLATE`          | `[OVERDUE] Event Notification`               | Template for alerting webhook title.                                                 |
| `--webhook.<name>.resolved-title-template` | `OVERDUE__WEBHOOK_<NAME>_RESOLVED_TITLE_TEMPLATE` | `[RESOLVED] [OVERDUE] Event Notification`    | Template for resolved webhook title.                                                 |
| `--webhook.<name>.text-template`           | `OVERDUE__WEBHOOK_<NAME>_TEXT_TEMPLATE`           | `Check-in "{{ .CheckInName }}" is overdue:`  | Template for alerting webhook text.                                                  |
| `--webhook.<name>.resolved-text-template`  | `OVERDUE__WEBHOOK_<NAME>_RESOLVED_TEXT_TEMPLATE`  | `Check-in "{{ .CheckInName }}" is resolved:` | Template for resolved webhook text.                                                  |
| `--webhook.<name>.headers`                 | `OVERDUE__WEBHOOK_<NAME>_HEADERS`                 | empty                                        | HTTP header in `KEY=VALUE` format. Can be repeated.                                  |
| `--webhook.<name>.custom-data`             | `OVERDUE__WEBHOOK_<NAME>_CUSTOM_DATA`             | empty                                        | Template data in `KEY=VALUE` format, available under `.CustomData`. Can be repeated. |
| `--webhook.<name>.template`                | `OVERDUE__WEBHOOK_<NAME>_TEMPLATE`                | required                                     | Path or `builtin:<name>` template for the webhook JSON body.                         |
| `--webhook.<name>.log-response`            | `OVERDUE__WEBHOOK_<NAME>_LOG_RESPONSE`            | `summary`                                    | Successful response logging mode: `summary`, `body`, `full`, or `none`.              |
| `--webhook.<name>.response-body-limit`     | `OVERDUE__WEBHOOK_<NAME>_RESPONSE_BODY_LIMIT`     | `4096`                                       | Maximum response body bytes to read for logs and errors.                             |

`<NAME>` is the uppercased target name. For example, `ops` becomes `OPS`.

### Webhook response logging

`--webhook.<name>.log-response` controls successful webhook logs:

| Mode      | Description                                                                        |
| --------- | ---------------------------------------------------------------------------------- |
| `summary` | Log status, status code, duration, and truncation state.                           |
| `body`    | Log summary fields and response body. JSON bodies are logged as structured values. |
| `full`    | Log summary fields, response body, and response headers.                           |
| `none`    | Suppress successful webhook response logs.                                         |

Non-2xx responses are treated as delivery failures.

The response body is included in the error, capped by `response-body-limit`.

### Webhook headers

Pass custom headers with `KEY=VALUE`:

```sh
--webhook.ops.headers='Authorization=Bearer token'
--webhook.ops.headers='X-Source=overdue'
```

With environment variables:

```sh
export OVERDUE__WEBHOOK_OPS_HEADERS='Authorization=Bearer token'
```

### Webhook custom data

Pass target-local template data with `KEY=VALUE` pairs:

```sh
--webhook.ops.custom-data=channel=#ops
--webhook.ops.custom-data=owner=platform
```

Templates can read those values through `.CustomData`:

```gotemplate
{{ .CustomData.channel }}
{{ index .CustomData "owner" }}
```

With environment variables:

```sh
export OVERDUE__WEBHOOK_OPS_CUSTOM_DATA='channel=#ops'
```

## Slack examples

### Slack incoming webhook

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --webhook.ops.url="$SLACK_WEBHOOK_URL" \
  --webhook.ops.template=builtin:slack-incoming-webhook \
  --webhook.ops.custom-data=channel=#alertmanager \
  --webhook.ops.send-resolved
```

With environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__WEBHOOK_OPS_URL="$SLACK_WEBHOOK_URL"
export OVERDUE__WEBHOOK_OPS_TEMPLATE=builtin:slack-incoming-webhook
export OVERDUE__WEBHOOK_OPS_CUSTOM_DATA=channel=#alertmanager
export OVERDUE__WEBHOOK_OPS_SEND_RESOLVED=true
```

### Slack `chat.postMessage`

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --webhook.ops.url=https://slack.com/api/chat.postMessage \
  --webhook.ops.headers="Authorization=Bearer $SLACK_TOKEN" \
  --webhook.ops.template=builtin:slack-chat-post-message \
  --webhook.ops.custom-data=channel=#alertmanager \
  --webhook.ops.send-resolved
```

The built-in Slack templates render the channel from `.CustomData.channel`. If no channel is configured, they use `#alertmanager`.

## Email notifications

Email notifications send HTML email via SMTP.

Example:

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --email.primary.smtp-host=smtp.example.com \
  --email.primary.smtp-port=587 \
  --email.primary.smtp-user="$SMTP_USER" \
  --email.primary.smtp-pass="$SMTP_PASS" \
  --email.primary.from=overdue@example.com \
  --email.primary.to=ops@example.com \
  --email.primary.headers='X-Trace=yes' \
  --email.primary.custom-data=owner=platform \
  --email.primary.template=builtin:email-html \
  --email.primary.send-resolved
```

With environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__EMAIL_PRIMARY_SMTP_HOST=smtp.example.com
export OVERDUE__EMAIL_PRIMARY_SMTP_PORT=587
export OVERDUE__EMAIL_PRIMARY_SMTP_USER="$SMTP_USER"
export OVERDUE__EMAIL_PRIMARY_SMTP_PASS="$SMTP_PASS"
export OVERDUE__EMAIL_PRIMARY_FROM=overdue@example.com
export OVERDUE__EMAIL_PRIMARY_TO=ops@example.com
export OVERDUE__EMAIL_PRIMARY_CUSTOM_DATA=owner=platform
export OVERDUE__EMAIL_PRIMARY_TEMPLATE=builtin:email-html
export OVERDUE__EMAIL_PRIMARY_SEND_RESOLVED=true
```

### Email flags

| Flag                                       | Environment variable pattern                      | Default                                      | Description                                                                          |
| ------------------------------------------ | ------------------------------------------------- | -------------------------------------------- | ------------------------------------------------------------------------------------ |
| `--email.<name>.smtp-host`                 | `OVERDUE__EMAIL_<NAME>_SMTP_HOST`                 | required                                     | SMTP host.                                                                           |
| `--email.<name>.smtp-port`                 | `OVERDUE__EMAIL_<NAME>_SMTP_PORT`                 | `587`                                        | SMTP port.                                                                           |
| `--email.<name>.smtp-user`                 | `OVERDUE__EMAIL_<NAME>_SMTP_USER`                 | required                                     | SMTP username.                                                                       |
| `--email.<name>.smtp-pass`                 | `OVERDUE__EMAIL_<NAME>_SMTP_PASS`                 | empty                                        | SMTP password.                                                                       |
| `--email.<name>.smtp-skip-insecure`        | `OVERDUE__EMAIL_<NAME>_SMTP_SKIP_INSECURE`        | `false`                                      | Skip SMTP TLS certificate verification.                                              |
| `--email.<name>.send-resolved`             | `OVERDUE__EMAIL_<NAME>_SEND_RESOLVED`             | `false`                                      | Send a resolved email after check-ins resume.                                        |
| `--email.<name>.subject-template`          | `OVERDUE__EMAIL_<NAME>_SUBJECT_TEMPLATE`          | status title                                 | Template for alerting email subject.                                                 |
| `--email.<name>.resolved-subject-template` | `OVERDUE__EMAIL_<NAME>_RESOLVED_SUBJECT_TEMPLATE` | status title                                 | Template for resolved email subject.                                                 |
| `--email.<name>.title-template`            | `OVERDUE__EMAIL_<NAME>_TITLE_TEMPLATE`            | `[OVERDUE] Event Notification`               | Template for alerting email body title.                                              |
| `--email.<name>.resolved-title-template`   | `OVERDUE__EMAIL_<NAME>_RESOLVED_TITLE_TEMPLATE`   | `[RESOLVED] [OVERDUE] Event Notification`    | Template for resolved email body title.                                              |
| `--email.<name>.text-template`             | `OVERDUE__EMAIL_<NAME>_TEXT_TEMPLATE`             | `Check-in "{{ .CheckInName }}" is overdue:`  | Template for alerting email body text.                                               |
| `--email.<name>.resolved-text-template`    | `OVERDUE__EMAIL_<NAME>_RESOLVED_TEXT_TEMPLATE`    | `Check-in "{{ .CheckInName }}" is resolved:` | Template for resolved email body text.                                               |
| `--email.<name>.from`                      | `OVERDUE__EMAIL_<NAME>_FROM`                      | required                                     | Email sender address.                                                                |
| `--email.<name>.to`                        | `OVERDUE__EMAIL_<NAME>_TO`                        | required                                     | Email recipient address. Can be repeated.                                            |
| `--email.<name>.headers`                   | `OVERDUE__EMAIL_<NAME>_HEADERS`                   | empty                                        | Email header in `KEY=VALUE` format. Can be repeated.                                 |
| `--email.<name>.custom-data`               | `OVERDUE__EMAIL_<NAME>_CUSTOM_DATA`               | empty                                        | Template data in `KEY=VALUE` format, available under `.CustomData`. Can be repeated. |
| `--email.<name>.template`                  | `OVERDUE__EMAIL_<NAME>_TEMPLATE`                  | required                                     | Path or `builtin:<name>` template for the email body.                                |

Overdue always adds `X-Mailer: overdue/<version>` to email notifications. User-provided `X-Mailer` values are ignored.

## Resolved notifications

Resolved notifications are disabled by default.

Enable them per target:

```sh
--webhook.ops.send-resolved
--email.primary.send-resolved
```

A resolved notification is queued when a new check-in arrives after the monitor was already in the `alerting` phase.

If the monitor was only `overdue`, no resolved notification is sent. The next check-in simply returns the monitor to `awaiting`.

## Notification retries

When a notification target fails, Overdue keeps retry state for that notification and retries it from the scheduler until delivery succeeds or the process stops.

For fan-out delivery, successful targets are not called again on the next retry. Only failed targets are retried.

Retry backoff starts at `1s` and is capped at `1m`.

Both alerting and resolved notifications keep their `.NotificationID` across retries.
