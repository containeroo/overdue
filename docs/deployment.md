# Deployment

Overdue is easiest to run as a container.

## Docker

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  ghcr.io/containeroo/overdue:latest
```

With authentication and Slack notifications:

```sh
docker run --rm -p 8080:8080 \
  -e OVERDUE__EXPECTED_EVERY=1m \
  -e OVERDUE__ALERTING_DELAY=10s \
  -e OVERDUE__AUTH_TOKEN="$OVERDUE_TOKEN" \
  -e OVERDUE__WEBHOOK_OPS_URL="$SLACK_WEBHOOK_URL" \
  -e OVERDUE__WEBHOOK_OPS_TEMPLATE=builtin:slack-incoming-webhook \
  -e OVERDUE__WEBHOOK_OPS_SEND_RESOLVED=true \
  ghcr.io/containeroo/overdue:latest
```

Check in:

```sh
curl -XPOST http://localhost:8080/check-in \
  -H "Authorization: Bearer $OVERDUE_TOKEN"
```

Check status:

```sh
curl http://localhost:8080/status \
  -H "Authorization: Bearer $OVERDUE_TOKEN"
```

## Docker Compose

The Docker Compose example lives in [`deploy/docker-compose.yaml`](../deploy/docker-compose.yaml).

Start it from the project root:

```sh
docker compose -f deploy/docker-compose.yaml up -d
```

The compose file uses environment variables for values that commonly differ between deployments. For example:

```sh
export OVERDUE__EXPECTED_EVERY=1m
export OVERDUE__ALERTING_DELAY=10s
export OVERDUE__AUTH_TOKEN=0123456789abcdef0123456789abcdef
```

An empty `OVERDUE__AUTH_TOKEN` disables authentication.

## Kubernetes

The Kubernetes manifests live in [`deploy/kubernetes/`](../deploy/kubernetes/).

Edit the secret before applying it:

```sh
cp deploy/kubernetes/secret.yaml /tmp/overdue-secret.yaml
$EDITOR /tmp/overdue-secret.yaml
kubectl apply -f /tmp/overdue-secret.yaml
```

Apply the workload manifests:

```sh
kubectl apply -f deploy/kubernetes/deployment.yaml
kubectl apply -f deploy/kubernetes/service.yaml
```

If you use the Prometheus Operator, apply the optional alerting rules:

```sh
kubectl apply -f deploy/kubernetes/prometheus/prometheus-rules.yaml
```

The rule labels may need to match your Prometheus rule selector. For example, kube-prometheus-stack deployments often select `PrometheusRule` objects by a release label.

Or apply the top-level Kubernetes directory after updating `secret.yaml` in place:

```sh
kubectl apply -f deploy/kubernetes/
```

The top-level directory command applies the core manifests only. Apply the optional Prometheus manifests separately when the Prometheus Operator CRDs are installed.

Expose the service through your ingress or gateway as usual.

If Overdue is mounted below a path prefix, set `OVERDUE__ROUTE_PREFIX` and make sure your ingress forwards the same prefix.
