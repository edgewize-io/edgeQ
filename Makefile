include Makefile.tools.mk

VERSION=$(shell git describe --tags --always)
BUILD_TIME := $(shell date -u +"%Y-%m-%d %H:%M:%S UTC" | tr -d " \t\n\r")
INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
CONTAINER_TOOL ?= docker
DRY_RUN=${DRY_RUN:-}

GO ?= go
GO_BUILD_LDFLAGS ?= -s -w -extldflags \"-static\" -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

REPO ?= mirrors.thingsdao.com/edgewize
TAG ?= latest

PROXY_IMAGE ?= $(REPO)/edge-qos-proxy:$(TAG)
BROKER_IMAGE ?= $(REPO)/edge-qos-broker:$(TAG)
CONTROLLER_MANAGER_IMAGE ?= $(REPO)/edge-qos-controller:$(TAG)
INIT_IPTABLES_IMAGE ?= $(REPO)/init-iptables:$(TAG)
APISERVER_IMAGE ?= $(REPO)/edge-qos-apiserver:$(TAG)

PLATFORMS ?= linux/arm64,linux/amd64


CRD_OPTIONS ?= "crd:allowDangerousTypes=true,generateEmbeddedObjectMeta=true"

.PHONY: proto
proto:
	API_PROTO_FILES=( $$(find {api,mindspore_serving} -name '*.proto') ); \
	for f in "$${API_PROTO_FILES[@]}"; do \
  		echo 'Processing '+$$f;\
		protoc \
        		-I ./ \
        		--gogo_out=plugins=grpc,:.\
        		--govalidators_out=gogoimport=true,:.\
		$$f;\
	done


# generate
test: clean
	#find . -type f -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort | uniq | xargs -I{} go test -v -vet=all -failfast -race {}
	ginkgo -v internal/broker/picker
	ginkgo -v tests/e2e

clean:
	rm -rf bin/

.PHONY: all
# generate all
all:
	make manifests
	make generate;

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

.PHONY: build-broker
build-broker:
	CGO_ENABLED=0 $(GO) build -ldflags="$(GO_BUILD_LDFLAGS)" -a -o broker cmd/broker/broker.go

.PHONY: build-proxy
build-proxy:
	CGO_ENABLED=0 $(GO) build -ldflags="$(GO_BUILD_LDFLAGS)" -a -o proxy cmd/proxy/proxy.go

.PHONY: build-apiserver
build-apiserver:
	CGO_ENABLED=0 $(GO) build -ldflags="$(GO_BUILD_LDFLAGS)" -a -o apiserver cmd/apiserver/apiserver.go

.PHONY: build-controller-manager
build-controller-manager:
	CGO_ENABLED=0 $(GO) build -ldflags="$(GO_BUILD_LDFLAGS)" -a -o controller-manager cmd/controller/controller-manager.go


##@Build
.PHONY: build-broker-image
build-broker-image:
	$(CONTAINER_TOOL) build -f build/broker/Dockerfile -t ${BROKER_IMAGE} .
	$(CONTAINER_TOOL) push ${BROKER_IMAGE}

.PHONY: build-proxy-image
build-proxy-image:
	$(CONTAINER_TOOL) build -f build/proxy/Dockerfile -t ${PROXY_IMAGE} .
	$(CONTAINER_TOOL) push ${PROXY_IMAGE}

.PHONY: build-init-image
build-init-image:
	$(CONTAINER_TOOL) build -f build/init-iptables/Dockerfile -t ${INIT_IPTABLES_IMAGE} .
	$(CONTAINER_TOOL) push ${INIT_IPTABLES_IMAGE}

.PHONY: build-controller-manager-image
build-controller-manager-image:
	$(CONTAINER_TOOL) build -f build/controller/Dockerfile -t ${CONTROLLER_MANAGER_IMAGE} .
	$(CONTAINER_TOOL) push ${CONTROLLER_MANAGER_IMAGE}

.PHONY: build-apiserver-image
build-apiserver-image:
	$(CONTAINER_TOOL) build -f build/apiserver/Dockerfile -t ${APISERVER_IMAGE} .
	$(CONTAINER_TOOL) push ${APISERVER_IMAGE}

.PHONY: build-and-push-all-images
build-and-push-all-images: build-broker-image build-proxy-image build-init-image build-controller-manager-image build-apiserver-image
	echo "all images completed"

.PHONY: build-test-images
build-test-images:
	$(CONTAINER_TOOL) build -f tests/mock/app/Dockerfile -t "edgewize/app-mock:latest" .
	$(CONTAINER_TOOL) build -f tests/mock/model/Dockerfile -t "edgewize/model-mock:latest" .

.PHONY: docker-buildx-test-images
docker-buildx-test-images:
	$(CONTAINER_TOOL) buildx build --builder mybuilder -t "harbor.dev.thingsdao.com/test/app:latest" --platform linux/amd64,linux/arm64  -f tests/mock/app/Dockerfile.buildx --push .
	$(CONTAINER_TOOL) buildx build --builder mybuilder -t "harbor.dev.thingsdao.com/test/model:latest" --platform linux/amd64,linux/arm64  -f tests/mock/model/Dockerfile.buildx --push .


.PHONY: docker-buildx-controller-image
docker-buildx-controller-image: ## Build and push docker image for the controller for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/controller/Dockerfile > Dockerfile.controller-cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${CONTROLLER_MANAGER_IMAGE} -f Dockerfile.controller-cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.controller-cross

.PHONY: docker-buildx-proxy-image
docker-buildx-proxy-image: ## Build and push docker image for the proxy for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/proxy/Dockerfile > Dockerfile.proxy-cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${PROXY_IMAGE} -f Dockerfile.proxy-cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.proxy-cross

.PHONY: docker-buildx-broker-image
docker-buildx-broker-image: ## Build and push docker image for the broker for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/broker/Dockerfile > Dockerfile.broker-cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${BROKER_IMAGE} -f Dockerfile.broker-cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.broker-cross

.PHONY: build-muti-architecture-images
build-muti-architecture-images: docker-buildx-controller-image docker-buildx-proxy-image docker-buildx-broker-image


# Generate manifests e.g. CRD, RBAC, etc.
.PHONY: manifests
manifests: controller-gen
	@$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=./pkg/apis/apps/... output:crd:dir=config/crds
	@$(CONTROLLER_GEN) rbac:roleName=edge-qos-role paths="./..." output:rbac:artifacts:config=config/rbac
	@$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths=./pkg/apis/apps/v1alpha1

	@cp config/crds/* charts/edge-qos/crds/

.PHONY: generate
generate: controller-gen
	hack/update-codegen.sh

