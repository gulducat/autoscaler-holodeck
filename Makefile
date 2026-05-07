MODULES := holodeck observer plugins/holodeck-apm plugins/nodesim-target

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
BINARIES := bin/nomad-nodesim $(addprefix bin/,$(MODULES))

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

.PHONY: env nomad nomad-status acl-bootstrap autoscaler job stop policy autoscaler

export NOMAD_ADDR ?= http://$(shell nomad node status -self -json 2>&1 | jq -r .HTTPAddr 2>/dev/null || echo 127.0.0.1:4646)
export NOMAD_TOKEN ?= 00000000-0000-0000-0000-000000000000

env:
	@echo export NOMAD_ADDR="$(NOMAD_ADDR)"
	@echo export NOMAD_TOKEN="$(NOMAD_TOKEN)"

sudo-nomad:
	sudo nomad version

nomad: sudo-nomad
	sudo nomad agent \
	  -config ./demo/nomad.hcl \
	  -data-dir $(PWD)/.nomad-data

nomad-status:
	nomad node status

acl-bootstrap:  
acl-bootstrap:
	@if nomad node status 2>&1 | grep -q 403 ; then \
	  echo "$(NOMAD_TOKEN)" | nomad acl bootstrap - ;\
	fi

autoscaler: nomad-status
	nomad-autoscaler agent -config demo/autoscaler/agent.hcl

job-policy: acl-bootstrap nomad-status
	nomad acl policy apply \
		-description="Read access for holodeck and observer tasks" \
		-job=holodeck \
		-namespace=default \
		holodeck-tasks \
		demo/jobs/holodeck-policy.hcl

job: job-policy
	nomad job run \
	  -var="nomad_addr=$(NOMAD_ADDR)" \
	  -var="sample_urls=nomad_metrics:$(NOMAD_ADDR)/v1/metrics" \
	  demo/jobs/holodeck.nomad.hcl

# Visibility

.PHONY: ui
ui: url-observer url-holodeck

url-observer:
url-holodeck:
url-%:
	@echo -e "$*: nomad service info $*\n$(shell nomad service info -t '{{range .}} * http://{{.Address}}:{{.Port}}\n{{end}}' $*)"

logs-observer:
logs-holodeck:
logs-nodesim:
logs-autoscaler:
logs-%:
	nomad logs -f -task $* -job holodeck

stop:
	nomad job stop holodeck

clean:
	nomad job stop holodeck || true
	nomad acl policy delete holodeck-tasks || true
	rm -rf bin/

