SHELL := /usr/bin/env bash

LOCALBIN ?= $(CURDIR)/bin
GO_TOOLCHAIN ?= go1.26.5
GOTOOLCHAIN_ENV := GOTOOLCHAIN=$(GO_TOOLCHAIN)

KUBEBUILDER ?= $(LOCALBIN)/kubebuilder
KUBEBUILDER_VERSION ?= v4.15.0
KUBEBUILDER_CHECKSUM ?= 9632ba818c35e10d9664f19090972ee5d667bdffbf4b83c34e8374c5e846afa2

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_GEN_VERSION ?= v0.21.0

KUSTOMIZE ?= $(LOCALBIN)/kustomize
KUSTOMIZE_VERSION ?= v5.8.1

SETUP_ENVTEST ?= $(LOCALBIN)/setup-envtest
SETUP_ENVTEST_VERSION ?= v0.24.1

.PHONY: tools
tools: $(KUBEBUILDER) $(CONTROLLER_GEN) $(KUSTOMIZE) $(SETUP_ENVTEST)

.PHONY: manifests
manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./api/..." paths="./internal/controller/..." output:crd:artifacts:config=config/crd/bases output:rbac:artifacts:config=config/rbac

.PHONY: generate
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object paths="./api/..."

.PHONY: test
test:
	$(GOTOOLCHAIN_ENV) go test ./...

.PHONY: build
build:
	$(GOTOOLCHAIN_ENV) go build -o manager ./cmd/manager

.PHONY: docker-build
docker-build:
	docker build -t ghcr.io/cognilabz/cognisecrets:latest .

.PHONY: render
render: $(KUSTOMIZE)
	$(KUSTOMIZE) build config/default

$(KUBEBUILDER):
	mkdir -p $(LOCALBIN)
	curl -fL https://github.com/kubernetes-sigs/kubebuilder/releases/download/$(KUBEBUILDER_VERSION)/kubebuilder_linux_amd64 -o $(KUBEBUILDER)
	echo "$(KUBEBUILDER_CHECKSUM)  $(KUBEBUILDER)" | sha256sum -c -
	chmod +x $(KUBEBUILDER)

$(CONTROLLER_GEN):
	mkdir -p $(LOCALBIN)
	$(GOTOOLCHAIN_ENV) GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

$(KUSTOMIZE):
	mkdir -p $(LOCALBIN)
	$(GOTOOLCHAIN_ENV) GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

$(SETUP_ENVTEST):
	mkdir -p $(LOCALBIN)
	$(GOTOOLCHAIN_ENV) GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)
