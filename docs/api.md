# HTTP API

Overdue exposes a small HTTP API.

| Method        | Path        | Description                | Auth required with `--auth-token` |
| ------------- | ----------- | -------------------------- | --------------------------------- |
| `GET`, `POST` | `/checkin` | Records a check-in.        | yes                               |
| `GET`         | `/status`   | Returns monitor state.     | yes                               |
| `GET`         | `/version`  | Returns build information. | no                                |
| `GET`, `POST` | `/healthz`  | Returns `ok`.              | no                                |

The check-in endpoint path is configurable with `--path`.

The other routes are mounted under `--route-prefix` when a route prefix is configured.

## `GET /checkin` and `POST /checkin`

Records a check-in.

`POST` is preferred because recording a check-in changes monitor state. `GET` is also accepted for simple integrations that can only call URLs.

Default response:

```json
{
  "status": "ok"
}
```

With `?details=true`:

```json
{
  "status": "ok",
  "checkInName": "default",
  "phase": "awaiting",
  "lastCheckIn": "2026-06-19T08:00:00Z",
  "expectedBy": "2026-06-19T08:01:00Z",
  "expectedEvery": "1m0s",
  "alertingAt": "2026-06-19T08:01:01Z",
  "alertingDelay": "1s",
  "alertingAfter": "1m1s"
}
```

The compact response acknowledges the command. Use the detailed response or the status endpoint when you need the resulting monitor phase and timing fields.

The query parameter accepts:

```text
details=true
details=1
details=yes
```

The `--response-details` flag makes check-in responses detailed by default.

## `GET /status`

Returns the current monitor state.

Compact response:

```json
{
  "lastCheckIn": "2026-06-19T08:00:00Z",
  "expectedBy": "2026-06-19T08:01:00Z",
  "alertingAt": "2026-06-19T08:01:01Z",
  "phase": "awaiting"
}
```

Before the first check-in:

```json
{
  "phase": "scheduled"
}
```

Detailed status:

```sh
curl 'http://localhost:8080/status?details=true'
```

Example response:

```json
{
  "checkInName": "default",
  "phase": "awaiting",
  "lastCheckIn": "2026-06-19T08:00:00Z",
  "expectedBy": "2026-06-19T08:01:00Z",
  "expectedEvery": "1m0s",
  "alertingAt": "2026-06-19T08:01:01Z",
  "alertingDelay": "1s",
  "alertingAfter": "1m1s",
  "notifications": {
    "status": "idle",
    "total": 0,
    "delivered": 0,
    "failed": 0,
    "pending": 0,
    "skipped": 0,
    "targets": []
  }
}
```

When notification targets are configured, detailed status includes one target entry per configured notifier:

```json
{
  "checkInName": "default",
  "phase": "alerting",
  "lastCheckIn": "2026-06-20T11:28:00Z",
  "expectedBy": "2026-06-20T11:29:00Z",
  "expectedEvery": "1m0s",
  "overdueSince": "2026-06-20T11:29:00Z",
  "overdueFor": "1m0s",
  "alertingAt": "2026-06-20T11:29:10Z",
  "alertingDelay": "10s",
  "alertingAfter": "1m10s",
  "alertingFor": "50s",
  "notifications": {
    "status": "partial_failure",
    "total": 2,
    "delivered": 1,
    "failed": 1,
    "pending": 1,
    "skipped": 0,
    "targets": [
      {
        "type": "webhook",
        "name": "teams",
        "status": "delivered",
        "lastAttemptAt": "2026-06-20T11:30:00Z",
        "lastDeliveredAt": "2026-06-20T11:30:00Z"
      },
      {
        "type": "email",
        "name": "email",
        "status": "failed",
        "lastAttemptAt": "2026-06-20T11:30:00Z"
      }
    ]
  }
}
```

Notification status values are:

| Status            | Meaning                                                                         |
| ----------------- | ------------------------------------------------------------------------------- |
| `idle`            | No notification delivery has been attempted yet.                                |
| `pending`         | A notification delivery attempt is in progress or waiting to retry.             |
| `delivered`       | Notification delivery completed successfully, or all remaining targets skipped. |
| `failed`          | At least one notification delivery failed and is still pending.                 |
| `skipped`         | A notification target intentionally skipped this event.                         |
| `partial_failure` | At least one target delivered while another target is still pending.            |

Notification status intentionally does not expose delivery error messages, because SMTP and webhook errors can include sensitive infrastructure details.

When the monitor is overdue or alerting, detailed status also includes elapsed operator fields:

```json
{
  "checkInName": "prometheus",
  "phase": "alerting",
  "lastCheckIn": "2026-06-20T22:51:06.308208+02:00",
  "expectedBy": "2026-06-20T22:51:11.308208+02:00",
  "expectedEvery": "5s",
  "overdueSince": "2026-06-20T22:51:11.308208+02:00",
  "overdueFor": "12s",
  "alertingAt": "2026-06-20T22:51:14.308208+02:00",
  "alertingDelay": "3s",
  "alertingAfter": "8s",
  "alertingFor": "9s"
}
```

## `GET /version`

Returns build information:

```json
{
  "version": "dev",
  "commit": "none"
}
```

## `GET /healthz`

Returns:

```text
ok
```

`POST /healthz` is also accepted.

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
GET  /overdue/checkin
POST /overdue/checkin
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




