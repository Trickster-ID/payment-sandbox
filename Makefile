SWAG_VERSION ?= v1.8.12
SWAG_CMD = go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
SWAG_ARGS = init -g app/cmd/main.go -o docs --parseDependency --parseInternal

MOCKERY_VERSION ?= v2.53.5
MOCKERY_CMD = go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

.PHONY: swag swagger mock coverage-services test-integration verify-batch10 verify-batch11 verify-iso verify-iso-ci drill-backup-restore

swag:
	$(SWAG_CMD) $(SWAG_ARGS)

swagger: swag

mock:
	$(MOCKERY_CMD) --config .mockery.yaml

coverage-services:
	go test -cover ./app/modules/.../services

test-integration:
	go test ./app/cmd -run TestIntegration -v

verify-batch10:
	go test ./...
	go test ./app/cmd -run TestIntegration -v
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
