MODULES := holodeck observer plugins/holodeck-apm plugins/nodesim-target

.PHONY: build test lint tidy clean visual run stop policy

build: bin/holodeck bin/observer bin/plugins/holodeck-apm bin/plugins/nodesim-target
	@find bin -type f

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

clean:
	rm -rf bin/

run: build
	nomad job run -var="bin_dir=$(CURDIR)/bin" jobs/holodeck.nomad.hcl

stop:
	nomad job stop -purge holodeck

policy:
	nomad acl policy apply \
		-description="Read access for holodeck and observer tasks" \
		-job=holodeck \
		-namespace=default \
		holodeck-tasks \
		jobs/holodeck-policy.hcl

bin/holodeck:
bin/observer:
bin/%:
	@mkdir -p bin
	go build -o bin/$* ./$*/cmd/$*

bin/plugins/holodeck-apm:
bin/plugins/nodesim-target:
bin/plugins/%:
	@mkdir -p bin/plugins
	go build -o bin/plugins/$* ./plugins/$*
