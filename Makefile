MODULES := holodeck observer plugins/holodeck-apm plugins/nodesim-target

.PHONY: build test lint tidy clean visual jobs stop policy autoscaler

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
	$(MAKE) stop || true
	rm -rf bin/

autoscaler: build
	nomad-autoscaler agent -config demo/autoscaler/agent.hcl

jobs: build
	nomad job run -var="bin_dir=$(CURDIR)/bin" demo/jobs/holodeck.nomad.hcl

jobs-sample: build
	nomad job run -var="bin_dir=$(CURDIR)/bin" -var="sample_urls=nomad_metrics:http://localhost:4646/v1/metrics" demo/jobs/holodeck.nomad.hcl

stop:
	nomad job stop -purge holodeck

policy:
	nomad acl policy apply \
		-description="Read access for holodeck and observer tasks" \
		-job=holodeck \
		-namespace=default \
		holodeck-tasks \
		demo/jobs/holodeck-policy.hcl

bin/plugins/holodeck-apm:
bin/plugins/nodesim-target:
bin/plugins/%:
	@mkdir -p bin/plugins
	go build -o bin/plugins/$* ./plugins/$*

bin/holodeck:
bin/observer:
bin/%:
	@mkdir -p bin
	go build -o bin/$* ./$*/cmd/$*
