SWAG_VERSION ?= v1.8.12
SWAG_CMD = go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
SWAG_ARGS = init -g app/cmd/main.go -o docs --parseDependency --parseInternal

MOCKERY_VERSION ?= v2.53.5
MOCKERY_CMD = go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

.PHONY: swag swagger mock

swag:
	$(SWAG_CMD) $(SWAG_ARGS)

swagger: swag

mock:
	$(MOCKERY_CMD) --config .mockery.yaml
