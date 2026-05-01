MODULES := $(shell find * -name 'go.mod' -exec dirname {} +)
MY_IP ?= 192.168.10.11

.PHONY: default docker build test lint tidy clean visual

default: build

test:
	@for m in $(MODULES); do echo "==> test $$m"; (cd $$m && go test ./...); done

lint:
	@for m in $(MODULES); do echo "==> lint $$m"; (cd $$m && golangci-lint run ./...); done

tidy:
	@for m in $(MODULES); do echo "==> tidy $$m"; (cd $$m && go mod tidy); done

visual:
	@echo "==> visual observer UI"
	@cd observer && VISUAL=1 go test ./... -run TestUI_Visual -v

# Build

BINDIR := $(CURDIR)/bin
BINARIES := $(addprefix bin/,$(MODULES))

docker:
	docker build -t holodeck:local .

build: $(BINARIES) # see bin/ targets down in the Build section
	@find bin -type f # just to show em

bin:
	mkdir -p bin

bin/nomad-autoscaler: bin
	# TODO: download nomad-autoscaler to bin/

nomad-nodesim:
	git clone https://github.com/gulducat/nomad-nodesim
	cd nomad-nodesim \
	  && git fetch origin feat/node-groups \
	  && git switch feat/node-groups

bin/nomad-nodesim: bin nomad-nodesim
	go build -C ./nomad-nodesim -o $(BINDIR)/nomad-nodesim .

bin/plugins/holodeck-apm:
bin/plugins/nodesim-target:
bin/plugins/%: bin
	go build -C ./plugins/$* -o $(BINDIR)/plugins/$* .

bin/holodeck:
bin/observer:
bin/%: bin
	go build -C ./$*/cmd/$* -o $(BINDIR)/$* .

# Demo

.PHONY: nomad autoscaler job stop policy autoscaler

nomad:
	nomad node status

autoscaler: nomad
	nomad-autoscaler agent -config demo/autoscaler/agent.hcl

job-policy: nomad
	nomad acl policy apply \
		-description="Read access for holodeck and observer tasks" \
		-job=holodeck \
		-namespace=default \
		holodeck-tasks \
		demo/jobs/holodeck-policy.hcl

job: nomad job-policy
	nomad job run \
	  -var="nomad_addr=http://$(MY_IP):4646" \
	  -var="sample_urls=nomad_metrics:http://$(MY_IP):4646/v1/metrics" \
	  demo/jobs/holodeck.nomad.hcl

stop:
	nomad job stop holodeck

clean:
	$(MAKE) stop || true
	rm -rf bin/

