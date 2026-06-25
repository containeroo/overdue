APP ?= overdue
IMAGE ?= containeroo/overdue
TAG ?= dev

TEST_FLAGS ?= -covermode=atomic -count=1 -parallel=4 -timeout=5m

BASE_URL ?= http://localhost:8080
ROUTE_PREFIX ?=
CHECKIN_PATH ?= /checkin
AUTH_TOKEN ?=
RUN_ARGS ?= --expected-every=5s --alerting-delay=3s

CURL ?= curl
CURL_FLAGS ?= --silent --show-error

AUTH_HEADER = $(if $(strip $(AUTH_TOKEN)),-H 'Authorization: Bearer $(AUTH_TOKEN)')

.DEFAULT_GOAL := help

# Default: no prefix. Can be overridden via `make patch VERSION_PREFIX=v`
VERSION_PREFIX ?= "v"

##@ Tagging

# Find the latest tag (with prefix filter if defined, default to 0.0.0 if none found)
# Lazy evaluation ensures fresh values on every run
LATEST_TAG = $(shell git tag --list "$(VERSION_PREFIX)*" --sort=-v:refname | head -n 1)
VERSION = $(shell [ -n "$(LATEST_TAG)" ] && echo $(LATEST_TAG) | sed "s/^$(VERSION_PREFIX)//" || echo "0.0.0")

patch: ## Create a new patch release (x.y.Z+1)
	@NEW_VERSION=$$(echo "$(VERSION)" | awk -F. '{printf "%d.%d.%d", $$1, $$2, $$3+1}') && \
	git tag "$(VERSION_PREFIX)$${NEW_VERSION}" && \
	echo "Tagged $(VERSION_PREFIX)$${NEW_VERSION}"

minor: ## Create a new minor release (x.Y+1.0)
	@NEW_VERSION=$$(echo "$(VERSION)" | awk -F. '{printf "%d.%d.0", $$1, $$2+1}') && \
	git tag "$(VERSION_PREFIX)$${NEW_VERSION}" && \
	echo "Tagged $(VERSION_PREFIX)$${NEW_VERSION}"

major: ## Create a new major release (X+1.0.0)
	@NEW_VERSION=$$(echo "$(VERSION)" | awk -F. '{printf "%d.0.0", $$1+1}') && \
	git tag "$(VERSION_PREFIX)$${NEW_VERSION}" && \
	echo "Tagged $(VERSION_PREFIX)$${NEW_VERSION}"

tag: ## Show latest tag
	@echo "Latest version: $(LATEST_TAG)"

push: ## Push tags to remote
	git push --tags

##@ Development

.PHONY: download
download: ## Download Go module dependencies.
	go mod download

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run unit tests.
	go test $(TEST_FLAGS) ./...

.PHONY: cover
cover: ## Display test coverage.
	go test -coverprofile=coverage.out $(TEST_FLAGS) ./...
	go tool cover -html=coverage.out

.PHONY: clean
clean: ## Clean up generated files.
	rm -f coverage.out coverage.html $(APP)

##@ Build and run

.PHONY: build
build: ## Build the binary.
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(APP) .

.PHONY: run
run: ## Run the service locally with overridable RUN_ARGS.
	go run . $(RUN_ARGS)

.PHONY: run-test
run-test: ## Run a local instance with example webhook and email settings.
	go run . \
		--expected-every=5s \
		--alerting-delay=3s \
		--name=prometheus \
		--webhook.ops.url=https://slack.com/api/chat.postMessage \
		--webhook.ops.headers="Authorization=Bearer $${SLACK_TOKEN}" \
		--webhook.ops.template=builtin:slack-chat-post-message \
		--webhook.ops.subject-template='{{ if .Resolved }}[RESOLVED]{{ else }}[OVERDUE]{{ end }} Check-in {{ .CheckInName }}' \
		--webhook.ops.custom-data=channel="$${SLACK_CHANNEL:-alertmanager}" \
		--webhook.ops.send-resolved \
		--email.ops.smtp-host=smtp.gmail.com \
		--email.ops.smtp-port=587 \
		--email.ops.smtp-user="$${EMAIL_USER}" \
		--email.ops.smtp-pass="$${EMAIL_PASS}" \
		--email.ops.from="$${EMAIL_FROM}" \
		--email.ops.to="$${EMAIL_TO}" \
		--email.ops.template=builtin:email-html \
		--email.ops.subject-template='{{ if .Resolved }}[RESOLVED]{{ else }}[OVERDUE]{{ end }} Check-in {{ .CheckInName }}' \
		--email.ops.send-resolved

##@ Endpoint smoke tests

.PHONY: check-in
check-in: ## Send a local check-in request. Example: make check-in | jq .
	@$(CURL) $(CURL_FLAGS) $(AUTH_HEADER) -X POST "$(BASE_URL)$(ROUTE_PREFIX)$(CHECKIN_PATH)"

.PHONY: check-in-details
check-in-details: ## Send a local check-in request with details. Example: make check-in-details | jq .
	@$(CURL) $(CURL_FLAGS) $(AUTH_HEADER) -X POST "$(BASE_URL)$(ROUTE_PREFIX)$(CHECKIN_PATH)?details=true"

.PHONY: status
status: ## Send a local status request. Example: make status | jq .
	@$(CURL) $(CURL_FLAGS) $(AUTH_HEADER) -X GET "$(BASE_URL)$(ROUTE_PREFIX)/status"

.PHONY: status-details
status-details: ## Send a local status request with details. Example: make status-details | jq .
	@$(CURL) $(CURL_FLAGS) $(AUTH_HEADER) -X GET "$(BASE_URL)$(ROUTE_PREFIX)/status?details=true"

.PHONY: version
version: ## Send a local version request. Example: make version | jq .
	@$(CURL) $(CURL_FLAGS) -X GET "$(BASE_URL)$(ROUTE_PREFIX)/version"

.PHONY: healthz
healthz: ## Send a local healthz GET request as JSON. Example: make healthz | jq .
	@body="$$( $(CURL) $(CURL_FLAGS) -X GET "$(BASE_URL)$(ROUTE_PREFIX)/healthz" )"; \
	printf '{"status":"%s"}\n' "$$body"

.PHONY: healthz-post
healthz-post: ## Send a local healthz POST request as JSON. Example: make healthz-post | jq .
	@body="$$( $(CURL) $(CURL_FLAGS) -X POST "$(BASE_URL)$(ROUTE_PREFIX)/healthz" )"; \
	printf '{"status":"%s"}\n' "$$body"

.PHONY: endpoints
endpoints: ## Test all local endpoints. Example: make endpoints | jq .
	@printf '%s\n' 'check-in' >&2
	@$(MAKE) --no-print-directory check-in
	@printf '%s\n' 'status' >&2
	@$(MAKE) --no-print-directory status
	@printf '%s\n' 'version' >&2
	@$(MAKE) --no-print-directory version
	@printf '%s\n' 'healthz' >&2
	@$(MAKE) --no-print-directory healthz

.PHONY: endpoints-details
endpoints-details: ## Test all local endpoints. Example: make endpoints-details | jq .
	@printf '%s\n' 'check-in-details' >&2
	@$(MAKE) --no-print-directory check-in-details
	@printf '%s\n' 'status-details' >&2
	@$(MAKE) --no-print-directory status-details
	@printf '%s\n' 'version' >&2
	@$(MAKE) --no-print-directory version
	@printf '%s\n' 'healthz-post' >&2
	@$(MAKE) --no-print-directory healthz-post

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
