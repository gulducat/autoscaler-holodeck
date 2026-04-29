MODULES := holodeck observer plugins/holodeck-apm plugins/nodesim-target

.PHONY: build test lint tidy visual

build:
	@for m in $(MODULES); do \
		echo "==> build $$m"; \
		(cd $$m && go build ./...); \
	done

test:
	@for m in $(MODULES); do \
		echo "==> test $$m"; \
		(cd $$m && go test ./...); \
	done

lint:
	@for m in $(MODULES); do \
		echo "==> lint $$m"; \
		(cd $$m && golangci-lint run ./...); \
	done

tidy:
	@for m in $(MODULES); do \
		echo "==> tidy $$m"; \
		(cd $$m && go mod tidy); \
	done

visual:
	@echo "==> visual observer UI"
	@cd observer && VISUAL=1 go test ./... -run TestUI_Visual -v
