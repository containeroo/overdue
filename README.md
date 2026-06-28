# Overdue

Overdue is a small HTTP check-in monitor.

It waits for periodic check-ins, moves through a simple lifecycle when a check-in is late, and sends notifications through configured webhook or email targets when the alerting deadline is reached.

## Lifecycle

```text
scheduled -> awaiting -> overdue -> alerting -> awaiting
```

- `scheduled`: no check-in has been received yet.
- `awaiting`: a check-in was received and the next deadline is active.
- `overdue`: the expected check-in deadline passed and the alerting delay is running.
- `alerting`: the alerting delay elapsed and an alert notification was queued.
- `resolved`: a resolved notification is queued when check-ins resume after alerting and the receiver has `send-resolved=true`.

## Quick start

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  ghcr.io/containeroo/overdue:latest
```

Send a check-in:

```sh
curl -X POST http://localhost:8080/checkin
```

Check status:

```sh
curl http://localhost:8080/status
```

## Configuration

Overdue is configured through CLI flags or environment variables.

Environment variables use the `OVERDUE__` prefix. Flag names are uppercased and dashes become underscores.

Examples:

```text
--expected-every                  -> OVERDUE__EXPECTED_EVERY
--alerting-delay                  -> OVERDUE__ALERTING_DELAY
--public-url                      -> OVERDUE__PUBLIC_URL
--webhook.ops.url                 -> OVERDUE__WEBHOOK_OPS_URL
--webhook.ops.custom-data         -> OVERDUE__WEBHOOK_OPS_CUSTOM_DATA
--email.primary.smtp-host         -> OVERDUE__EMAIL_PRIMARY_SMTP_HOST
--email.primary.from              -> OVERDUE__EMAIL_PRIMARY_FROM
```

### Core flags

| Flag                  | Environment variable         | Default    | Description                                                                |
| --------------------- | ---------------------------- | ---------- | -------------------------------------------------------------------------- |
| `--listen-address`    | `OVERDUE__LISTEN_ADDRESS`    | `:8080`    | HTTP server listen address.                                                |
| `--route-prefix`      | `OVERDUE__ROUTE_PREFIX`      | empty      | Optional path prefix when Overdue is served below a sub-path.              |
| `--public-url`        | `OVERDUE__PUBLIC_URL`        | empty      | Public base URL used in notification template links.                       |
| `--name`              | `OVERDUE__NAME`              | `default`  | Name of the check-in monitor used in responses and notifications.          |
| `--path`              | `OVERDUE__PATH`              | `/checkin` | Route used to receive check-ins.                                           |
| `--expected-every`    | `OVERDUE__EXPECTED_EVERY`    | required   | Maximum time between check-ins.                                            |
| `--alerting-delay`    | `OVERDUE__ALERTING_DELAY`    | required   | Extra time after the expected deadline before alerting.                    |
| `--start-active`      | `OVERDUE__START_ACTIVE`      | `false`    | Activate the monitor at startup instead of waiting for the first check-in. |
| `--allow-get-checkin` | `OVERDUE__ALLOW_GET_CHECKIN` | `false`    | Also accept `GET` requests on the check-in route.                          |
| `--response-details`  | `OVERDUE__RESPONSE_DETAILS`  | `false`    | Return detailed timing fields from check-in responses by default.          |
| `--auth-token`        | `OVERDUE__AUTH_TOKEN`        | empty      | Optional bearer token required for check-in and status requests.           |
| `--debug`             | `OVERDUE__DEBUG`             | `false`    | Enable debug logging.                                                      |
| `--log-format`        | `OVERDUE__LOG_FORMAT`        | `json`     | Log format: `json` or `text`.                                              |

## Notifications

Overdue supports multiple named webhook and email targets.

Each target is configured as a dynamic group instance:

```text
webhook.<name>.<field>
email.<name>.<field>
```

Examples:

```text
--webhook.ops.url=https://hooks.slack.com/services/...
--email.primary.smtp-host=smtp.example.com
```

If no notification targets are configured, Overdue still runs and records status, but no notifications are sent.

### Webhook flags

| Flag pattern                           | Environment variable pattern                  | Default         | Description                                                     |
| -------------------------------------- | --------------------------------------------- | --------------- | --------------------------------------------------------------- |
| `--webhook.<name>.url`                 | `OVERDUE__WEBHOOK_<NAME>_URL`                 | required        | Webhook URL.                                                    |
| `--webhook.<name>.method`              | `OVERDUE__WEBHOOK_<NAME>_METHOD`              | `POST`          | HTTP method: `POST`, `PUT`, `PATCH`, or `DELETE`.               |
| `--webhook.<name>.timeout`             | `OVERDUE__WEBHOOK_<NAME>_TIMEOUT`             | `10s`           | HTTP request timeout.                                           |
| `--webhook.<name>.skip-insecure`       | `OVERDUE__WEBHOOK_<NAME>_SKIP_INSECURE`       | `false`         | Skip TLS certificate verification.                              |
| `--webhook.<name>.send-resolved`       | `OVERDUE__WEBHOOK_<NAME>_SEND_RESOLVED`       | `false`         | Send resolved notifications to this receiver.                   |
| `--webhook.<name>.subject-template`    | `OVERDUE__WEBHOOK_<NAME>_SUBJECT_TEMPLATE`    | default subject | Subject/title template.                                         |
| `--webhook.<name>.headers`             | `OVERDUE__WEBHOOK_<NAME>_HEADERS`             | empty           | HTTP headers in `KEY=VALUE` format.                             |
| `--webhook.<name>.custom-data`         | `OVERDUE__WEBHOOK_<NAME>_CUSTOM_DATA`         | empty           | Custom template data in `KEY=VALUE` format.                     |
| `--webhook.<name>.template`            | `OVERDUE__WEBHOOK_<NAME>_TEMPLATE`            | required        | Body template path or `builtin:<name>`.                         |
| `--webhook.<name>.log-response`        | `OVERDUE__WEBHOOK_<NAME>_LOG_RESPONSE`        | `summary`       | Webhook response logging: `summary`, `body`, `full`, or `none`. |
| `--webhook.<name>.response-body-limit` | `OVERDUE__WEBHOOK_<NAME>_RESPONSE_BODY_LIMIT` | `4096`          | Maximum response body bytes to read for logs and errors.        |

### Email flags

| Flag pattern                        | Environment variable pattern               | Default         | Description                                   |
| ----------------------------------- | ------------------------------------------ | --------------- | --------------------------------------------- |
| `--email.<name>.smtp-host`          | `OVERDUE__EMAIL_<NAME>_SMTP_HOST`          | required        | SMTP host.                                    |
| `--email.<name>.smtp-port`          | `OVERDUE__EMAIL_<NAME>_SMTP_PORT`          | `587`           | SMTP port.                                    |
| `--email.<name>.smtp-user`          | `OVERDUE__EMAIL_<NAME>_SMTP_USER`          | empty           | SMTP username.                                |
| `--email.<name>.smtp-pass`          | `OVERDUE__EMAIL_<NAME>_SMTP_PASS`          | empty           | SMTP password.                                |
| `--email.<name>.smtp-skip-insecure` | `OVERDUE__EMAIL_<NAME>_SMTP_SKIP_INSECURE` | `false`         | Skip SMTP TLS certificate verification.       |
| `--email.<name>.send-resolved`      | `OVERDUE__EMAIL_<NAME>_SEND_RESOLVED`      | `false`         | Send resolved notifications to this receiver. |
| `--email.<name>.subject-template`   | `OVERDUE__EMAIL_<NAME>_SUBJECT_TEMPLATE`   | default subject | Email subject template.                       |
| `--email.<name>.from`               | `OVERDUE__EMAIL_<NAME>_FROM`               | required        | Sender address.                               |
| `--email.<name>.to`                 | `OVERDUE__EMAIL_<NAME>_TO`                 | required        | Recipient address. May be repeated.           |
| `--email.<name>.headers`            | `OVERDUE__EMAIL_<NAME>_HEADERS`            | empty           | Email headers in `KEY=VALUE` format.          |
| `--email.<name>.custom-data`        | `OVERDUE__EMAIL_<NAME>_CUSTOM_DATA`        | empty           | Custom template data in `KEY=VALUE` format.   |
| `--email.<name>.template`           | `OVERDUE__EMAIL_<NAME>_TEMPLATE`           | required        | Body template path or `builtin:<name>`.       |

## Webhook examples

### Slack incoming webhook

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  -e OVERDUE__PUBLIC_URL=https://overdue.example.com \
  -e OVERDUE__WEBHOOK_OPS_URL="$SLACK_WEBHOOK_URL" \
  -e OVERDUE__WEBHOOK_OPS_TEMPLATE=builtin:slack-incoming-webhook \
  -e OVERDUE__WEBHOOK_OPS_CUSTOM_DATA=channel=#alertmanager \
  -e OVERDUE__WEBHOOK_OPS_SEND_RESOLVED=true \
  ghcr.io/containeroo/overdue:latest
```

### Generic JSON webhook

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  -e OVERDUE__WEBHOOK_OPS_URL=https://example.com/webhook \
  -e OVERDUE__WEBHOOK_OPS_TEMPLATE=/etc/overdue/webhook.tmpl \
  -v "$PWD/webhook.tmpl:/etc/overdue/webhook.tmpl:ro" \
  ghcr.io/containeroo/overdue:latest
```

Example `webhook.tmpl`:

```json
{
  "title": {{ .Title | json }},
  "text": {{ .Text | json }},
  "status": {{ .Status | json }},
  "checkInName": {{ .CheckInName | json }},
  "resolved": {{ .Resolved | json }}
}
```

## Email example

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  -e OVERDUE__PUBLIC_URL=https://overdue.example.com \
  -e OVERDUE__EMAIL_OPS_SMTP_HOST=smtp.example.com \
  -e OVERDUE__EMAIL_OPS_SMTP_PORT=587 \
  -e OVERDUE__EMAIL_OPS_SMTP_USER=overdue@example.com \
  -e OVERDUE__EMAIL_OPS_SMTP_PASS="$SMTP_PASSWORD" \
  -e OVERDUE__EMAIL_OPS_FROM=overdue@example.com \
  -e OVERDUE__EMAIL_OPS_TO=ops@example.com \
  -e OVERDUE__EMAIL_OPS_TEMPLATE=builtin:email-html \
  -e OVERDUE__EMAIL_OPS_SEND_RESOLVED=true \
  ghcr.io/containeroo/overdue:latest
```

`OVERDUE__EMAIL_OPS_FROM` is required. If it is missing, notification setup or delivery fails because the email target cannot build a valid message.

## Templates

Notification payloads are rendered with Go templates.

Overdue includes built-in templates:

```text
builtin:email-html
builtin:slack-incoming-webhook
builtin:slack-chat-post-message
```

Custom templates can be mounted into the container and referenced by path:

```sh
-e OVERDUE__WEBHOOK_OPS_TEMPLATE=/etc/overdue/slack.tmpl
```

Webhook templates must render valid JSON. Email templates may render text or HTML.

Templates use strict missing-key behavior. Missing fields fail during startup validation or delivery instead of rendering silently.

### Template data

Templates receive the following data:

| Field             | Type                | Description                                                                    |
| ----------------- | ------------------- | ------------------------------------------------------------------------------ |
| `.IncidentID`     | `string`            | Stable ID for one overdue incident.                                            |
| `.NotificationID` | `string`            | Stable ID for this concrete notification.                                      |
| `.CheckInName`    | `string`            | Configured check-in name.                                                      |
| `.LastCheckIn`    | `time.Time`         | Last received check-in time.                                                   |
| `.ExpectedBy`     | `time.Time`         | Time when the next check-in was expected.                                      |
| `.OverdueSince`   | `time.Time`         | Time when the check-in became overdue.                                         |
| `.AlertingAt`     | `time.Time`         | Time when alerting starts.                                                     |
| `.Now`            | `time.Time`         | Time the notification event was created.                                       |
| `.Phase`          | `string`            | Monitor phase.                                                                 |
| `.Status`         | `string`            | Notification status: `alerting` or `resolved`.                                 |
| `.Resolved`       | `bool`              | Whether this is a resolved notification.                                       |
| `.Subject`        | `string`            | Rendered subject.                                                              |
| `.Title`          | `string`            | Title for the notification. Currently the rendered subject or a default title. |
| `.Text`           | `string`            | Default plain text summary.                                                    |
| `.Receiver`       | `string`            | Receiver name.                                                                 |
| `.Vars`           | `map[string]any`    | Public receiver variables. Custom data is also exposed here.                   |
| `.CustomData`     | `map[string]string` | Custom data configured with `custom-data`.                                     |
| `.App.Version`    | `string`            | Overdue version.                                                               |
| `.App.SiteRoot`   | `string`            | Public base URL from `--public-url`.                                           |
| `.App.CheckInURL` | `string`            | Public check-in URL when `--public-url` is configured.                         |
| `.App.StatusURL`  | `string`            | Public status URL when `--public-url` is configured.                           |

### Custom data

Custom data is configured with `KEY=VALUE` values:

```sh
-e OVERDUE__WEBHOOK_OPS_CUSTOM_DATA=channel=#alertmanager
```

It is available in templates as both `.CustomData` and `.Vars`.

Recommended access:

```gotemplate
{{ index .CustomData "channel" }}
{{ index .Vars "channel" }}
```

Example with a default:

```gotemplate
{{ index .CustomData "channel" | default "alertmanager" | withPrefix "#" }}
```

## Subject templates

The default subject template is:

```gotemplate
{{ if .Resolved }}[RESOLVED]{{ else }}[OVERDUE]{{ end }} Event Notification
```

Override it per receiver:

```sh
-e OVERDUE__WEBHOOK_OPS_SUBJECT_TEMPLATE='{{ if .Resolved }}[OK]{{ else }}[ALERT]{{ end }} {{ .CheckInName }}'
-e OVERDUE__EMAIL_OPS_SUBJECT_TEMPLATE='{{ if .Resolved }}[OK]{{ else }}[ALERT]{{ end }} {{ .CheckInName }}'
```

## HTTP API

### `POST /checkin`

Records a check-in.

```sh
curl -X POST http://localhost:8080/checkin
```

Compact response:

```json
{ "status": "ok" }
```

Detailed response:

```sh
curl -X POST 'http://localhost:8080/checkin?details=true'
```

### `GET /checkin`

Disabled by default. Enable `--allow-get-checkin` only for simple uptime systems that cannot send `POST` requests. When enabled, `GET /checkin` also records a check-in.

### `GET /status`

Returns the current monitor state.

```sh
curl http://localhost:8080/status
```

Detailed status:

```sh
curl 'http://localhost:8080/status?details=true'
```

### `GET /healthz` and `POST /healthz`

Liveness probe.

```sh
curl http://localhost:8080/healthz
```

### `GET /version`

Returns build version and commit.

```sh
curl http://localhost:8080/version
```

### `GET /metrics`

Prometheus metrics endpoint. This endpoint is intentionally not protected by `--auth-token`, because Prometheus commonly scrapes without application bearer tokens. Do not expose it directly to the public internet unless a reverse proxy, firewall, or network policy restricts access.

```sh
curl http://localhost:8080/metrics
```

## Authentication

Set `--auth-token` or `OVERDUE__AUTH_TOKEN` to require a bearer token for check-in and status requests. The metrics, health, readiness, and version endpoints remain unauthenticated; protect them at the reverse proxy or network layer when exposed outside a trusted network.

```sh
curl -H "Authorization: Bearer $OVERDUE_TOKEN" \
  -X POST http://localhost:8080/checkin
```

## Route prefix

Use `--route-prefix` when Overdue is served below a path prefix.

```sh
-e OVERDUE__ROUTE_PREFIX=/watchdog
```

With this prefix, endpoints are served below `/watchdog`:

```text
/watchdog/checkin
/watchdog/status
/watchdog/metrics
/watchdog/healthz
```

Set `--public-url` to the externally reachable base URL if you want templates to include correct links:

```sh
-e OVERDUE__PUBLIC_URL=https://overdue.example.com/watchdog
```

## Docker Compose

```yaml
---
name: overdue

services:
  overdue:
    image: ghcr.io/containeroo/overdue:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      OVERDUE__EXPECTED_EVERY: 1m
      OVERDUE__ALERTING_DELAY: 10s
      OVERDUE__PUBLIC_URL: https://overdue.example.com
      OVERDUE__WEBHOOK_OPS_URL: "${SLACK_WEBHOOK_URL}"
      OVERDUE__WEBHOOK_OPS_TEMPLATE: builtin:slack-incoming-webhook
      OVERDUE__WEBHOOK_OPS_CUSTOM_DATA: channel=#alertmanager
      OVERDUE__WEBHOOK_OPS_SEND_RESOLVED: "true"
```

## Development

Run tests:

```sh
make test
```

Run locally:

```sh
go run . \
  --expected-every=1m \
  --alerting-delay=10s
```

Run locally with an email receiver:

```sh
go run . \
  --expected-every=1m \
  --alerting-delay=10s \
  --email.ops.smtp-host=smtp.example.com \
  --email.ops.from=overdue@example.com \
  --email.ops.to=ops@example.com \
  --email.ops.template=builtin:email-html
```

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](LICENSE) file for details.
