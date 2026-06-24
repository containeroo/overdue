# Configuration

Overdue can be configured with CLI flags or environment variables.

CLI flags override environment values.

Environment variables use the `OVERDUE__` prefix. Flag names are uppercased and dashes become underscores.

```text
--expected-every        -> OVERDUE__EXPECTED_EVERY
--alerting-delay        -> OVERDUE__ALERTING_DELAY
--check-in-name         -> OVERDUE__CHECK_IN_NAME
--response-details      -> OVERDUE__RESPONSE_DETAILS
```

Dynamic notification flags include the target name:

```text
--webhook.ops.url                    -> OVERDUE__WEBHOOK_OPS_URL
--webhook.ops.method                 -> OVERDUE__WEBHOOK_OPS_METHOD
--webhook.ops.custom-data            -> OVERDUE__WEBHOOK_OPS_CUSTOM_DATA
--webhook.ops.send-resolved          -> OVERDUE__WEBHOOK_OPS_SEND_RESOLVED
--email.primary.smtp-host            -> OVERDUE__EMAIL_PRIMARY_SMTP_HOST
--email.primary.smtp-skip-insecure   -> OVERDUE__EMAIL_PRIMARY_SMTP_SKIP_INSECURE
--email.primary.custom-data          -> OVERDUE__EMAIL_PRIMARY_CUSTOM_DATA
```

## Required settings

| Flag               | Environment variable      | Description                                                       |
| ------------------ | ------------------------- | ----------------------------------------------------------------- |
| `--expected-every` | `OVERDUE__EXPECTED_EVERY` | Maximum allowed time between check-ins.                           |
| `--alerting-delay` | `OVERDUE__ALERTING_DELAY` | Extra time after the expected deadline before notifications fire. |

## Core flags

| Flag                 | Environment variable        | Default     | Description                                                                |
| -------------------- | --------------------------- | ----------- | -------------------------------------------------------------------------- |
| `--listen-address`   | `OVERDUE__LISTEN_ADDRESS`   | `:8080`     | HTTP server listen address.                                                |
| `--route-prefix`     | `OVERDUE__ROUTE_PREFIX`     | empty       | Path prefix to mount the service under.                                    |
| `--check-in-name`    | `OVERDUE__CHECK_IN_NAME`    | `default`   | Name of the check-in monitor used in notifications.                        |
| `--check-in-path`    | `OVERDUE__CHECK_IN_PATH`    | `/check-in` | Route used to receive check-ins.                                           |
| `--expected-every`   | `OVERDUE__EXPECTED_EVERY`   | required    | Maximum time between check-ins.                                            |
| `--alerting-delay`   | `OVERDUE__ALERTING_DELAY`   | required    | Extra time after the expected deadline before notifications fire.          |
| `--start-active`     | `OVERDUE__START_ACTIVE`     | `false`     | Activate the monitor at startup instead of waiting for the first check-in. |
| `--response-details` | `OVERDUE__RESPONSE_DETAILS` | `false`     | Return detailed timing fields from check-in responses by default.          |
| `--auth-token`       | `OVERDUE__AUTH_TOKEN`       | empty       | Optional bearer token required for check-in and status requests.           |
| `--debug`            | `OVERDUE__DEBUG`            | `false`     | Enable debug logging.                                                      |
| `--log-format`       | `OVERDUE__LOG_FORMAT`       | `json`      | Log format: `json` or `text`.                                              |

## Timing

The two timing settings are intentionally separate.

| Setting            | Meaning                                                           |
| ------------------ | ----------------------------------------------------------------- |
| `--expected-every` | Maximum allowed time between check-ins.                           |
| `--alerting-delay` | Extra time after the expected deadline before notifications fire. |

Example:

```text
last check-in:    12:00:00
expected every:   5m
alerting delay:   30s

expected by:      12:05:00
overdue since:    12:05:00
alerting at:      12:05:30
```

At `12:05:00`, the monitor becomes `overdue`.

At `12:05:30`, the monitor becomes `alerting` and an alerting notification is queued for delivery.

If a new check-in arrives after alerting, Overdue returns to `awaiting`. Notification targets with `send-resolved` enabled also receive a resolved notification. Alerting and resolved notifications use the same retry path.

## Start active

By default, Overdue starts in the `scheduled` phase and waits for the first check-in before scheduling deadlines.

Use `--start-active` to activate the monitor at startup:

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --start-active
```

With environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__START_ACTIVE=true
```

This behaves like a check-in was received at startup time.

## Route prefix

Use `--route-prefix` to mount the service under a path prefix.

```sh
overdue \
  --route-prefix=/overdue \
  --expected-every=1m \
  --alerting-delay=10s
```

Routes become:

```text
GET /overdue/check-in
POST /overdue/check-in
GET  /overdue/status
GET  /overdue/version
GET  /overdue/healthz
POST /overdue/healthz
```

`--route-prefix` may also be given as a full URL. Only the path is used.

```sh
--route-prefix=https://example.com/overdue/
```

This becomes:

```text
/overdue
```

## Authentication

Set `--auth-token` to require bearer-token auth for `/check-in` and `/status`.

```sh
overdue \
  --expected-every=1m \
  --alerting-delay=10s \
  --auth-token=0123456789abcdef0123456789abcdef
```

With environment variables:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__AUTH_TOKEN=0123456789abcdef0123456789abcdef
```

Send authorized requests:

```sh
curl -XPOST http://localhost:8080/check-in \
  -H 'Authorization: Bearer 0123456789abcdef0123456789abcdef'

curl http://localhost:8080/status \
  -H 'Authorization: Bearer 0123456789abcdef0123456789abcdef'
```

`/healthz` and `/version` do not require the token.

An empty auth token disables authentication.

Non-empty auth tokens must follow a small safety policy:

| Rule          | Value                                      |
| ------------- | ------------------------------------------ |
| Minimum       | 32 characters                              |
| Maximum       | 4096 characters                            |
| Character set | Printable ASCII only, from `!` through `~` |
| Whitespace    | Not allowed anywhere                       |

This rejects leading spaces, trailing spaces, tabs, newlines, control characters, and non-ASCII Unicode characters. Tokens are validated as provided and are not silently trimmed.

Generate a suitable token with:

```sh
openssl rand -hex 32
```

Use it through the environment when possible:

```sh
export OVERDUE__AUTH_TOKEN="$(openssl rand -hex 32)"
```
