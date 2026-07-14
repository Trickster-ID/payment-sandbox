ifneq (,$(wildcard .env))
  include .env
  export
endif

SWAG_VERSION ?= v1.8.12
SWAG_CMD = go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
SWAG_ARGS = init -g app/cmd/main.go -o docs --parseDependency --parseInternal

MOCKERY_CMD = go run github.com/vektra/mockery/v2

# ── Performance test targets ──────────────────────────────────────────────────
# K6_* vars are sourced from .env above; these are fallbacks for CI environments.
K6_BASE_URL             ?= http://127.0.0.1:8080
K6_ADMIN_EMAIL          ?=
K6_ADMIN_PASSWORD       ?=
K6_OAUTH2_CLIENT_ID     ?=
K6_OAUTH2_CLIENT_SECRET ?=
K6_REPORT_DIR           ?= docs/k6/reports

K6_ENV = K6_BASE_URL=$(K6_BASE_URL) \
         K6_ADMIN_EMAIL=$(K6_ADMIN_EMAIL) \
         K6_ADMIN_PASSWORD=$(K6_ADMIN_PASSWORD) \
         K6_OAUTH2_CLIENT_ID=$(K6_OAUTH2_CLIENT_ID) \
         K6_OAUTH2_CLIENT_SECRET=$(K6_OAUTH2_CLIENT_SECRET) \
         K6_REPORT_DIR=$(K6_REPORT_DIR)

.PHONY: perf-smoke perf-baseline perf-stress perf-soak perf-full-coverage perf-open-last-report perf-clean-reports

perf-smoke:
	@mkdir -p $(K6_REPORT_DIR)
	@echo "▶ Running smoke test…"
	$(K6_ENV) k6 run docs/k6/scripts/smoke.js
	@echo "✔ Reports written to $(K6_REPORT_DIR)"

perf-baseline:
	@mkdir -p $(K6_REPORT_DIR)
	@echo "▶ Running baseline test…"
	$(K6_ENV) k6 run docs/k6/scripts/baseline.js
	@echo "✔ Reports written to $(K6_REPORT_DIR)"

perf-stress:
	@mkdir -p $(K6_REPORT_DIR)
	@echo "▶ Running stress test…"
	$(K6_ENV) k6 run docs/k6/scripts/stress.js
	@echo "✔ Reports written to $(K6_REPORT_DIR)"

perf-soak:
	@mkdir -p $(K6_REPORT_DIR)
	@echo "▶ Running soak test (long-running)…"
	$(K6_ENV) k6 run docs/k6/scripts/soak.js
	@echo "✔ Reports written to $(K6_REPORT_DIR)"

perf-full-coverage:
	@mkdir -p $(K6_REPORT_DIR)
	@echo "▶ Running full-coverage test…"
	$(K6_ENV) k6 run docs/k6/scripts/full-coverage.js
	@echo "✔ Reports written to $(K6_REPORT_DIR)"

perf-open-last-report:
	@html=$$(ls -t $(K6_REPORT_DIR)/*.html 2>/dev/null | head -1); \
	if [ -n "$$html" ]; then open "$$html"; else echo "No report found in $(K6_REPORT_DIR)"; fi

perf-clean-reports:
	@echo "Removing all reports under $(K6_REPORT_DIR)…"
	@find $(K6_REPORT_DIR) -mindepth 1 -not -name ".gitkeep" -delete
	@echo "Done."

# ──────────────────────────────────────────────────────────────────────────────

.PHONY: swag swagger mock wire init clean-generated coverage-services test-integration test-e2e-api verify-batch10 verify-batch11 verify-iso verify-iso-ci drill-backup-restore

# ── Code generation targets ───────────────────────────────────────────────────

swag:
	$(SWAG_CMD) $(SWAG_ARGS)

swagger: swag

mock:
	$(MOCKERY_CMD) --config .mockery.yaml

wire:
	cd app/cmd && go run github.com/google/wire/cmd/wire

clean-generated:
	@echo "▶ Removing generated mocks…"
	@find ./app -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	@echo "▶ Removing wire_gen.go…"
	@rm -f app/cmd/wire_gen.go
	@echo "▶ Removing swagger docs…"
	@rm -f docs/docs.go docs/swagger.json docs/swagger.yaml
	@echo "✔ All generated files removed."

init: clean-generated
	go mod tidy
	@echo ""
	@echo "▶ Regenerating mocks…"
	@$(MAKE) mock
	@echo ""
	@echo "▶ Regenerating swagger…"
	@$(MAKE) swag
	@echo ""
	@echo "▶ Regenerating wire…"
	@$(MAKE) wire
	@echo ""
	@echo "✔ All generated files recreated."

coverage-services:
	go test -cover ./app/modules/.../services

test-integration:
	go test ./app/cmd -run TestIntegration -v

test-e2e-api:
	go test ./app/cmd -run TestE2EAPI -v

verify-batch10:
	go test ./...
	go test ./app/cmd -run TestIntegration -v
	go test ./app/cmd -run TestE2EAPI -v
	go test ./app/modules/admin/services ./app/modules/invoice/services
	go test -cover ./app/modules/.../services

verify-batch11:
	go test ./...
	go test ./app/cmd -run TestNewRouter_RegistersExpectedRoutes -v
	./misc/verify/batch11-query-plans.sh

verify-iso:
	./misc/verify/iso-readiness.sh

verify-iso-ci:
	./misc/verify/iso-readiness-ci.sh

drill-backup-restore:
	./misc/ops/drill-backup-restore.sh
