GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)
INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
CONTAINER_TOOL ?= docker
DRY_RUN=${DRY_RUN:-}

REPO ?= kubesphere
TAG ?= latest

PROXY_IMAGE ?= $(REPO)/model-mesh-proxy:$(TAG)
BROKER_IMAGE ?= $(REPO)/model-mesh-broker:$(TAG)
MSC_IMAGE ?= $(REPO)/model-mesh-msc:$(TAG)

CONTROLLER_TOOLS_VERSION ?= v0.13.0
PLATFORMS ?= linux/arm64,linux/amd64

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

CRD_OPTIONS ?= "crd:allowDangerousTypes=true"

MANIFESTS="template/*"

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


.PHONY: init
# init env
init:
	go get -d -u  github.com/tkeel-io/tkeel-interface/openapi
	go get -d -u  github.com/tkeel-io/kit
	go get -d -u  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.7.0

	go install github.com/ocavue/protoc-gen-typescript@latest
	go install  github.com/tkeel-io/tkeel-interface/tool/cmd/artisan@latest
	go install  google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
	go install  google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1.0
	go install  github.com/tkeel-io/tkeel-interface/protoc-gen-go-http@latest
	go install  github.com/tkeel-io/tkeel-interface/protoc-gen-go-errors@latest
	go install  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.7.0

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


.PHONY: build
# build
build:
	mkdir -p bin/ && go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...

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
	make api;
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

##@Build
.PHONY: build-broker-image
build-broker-image:
	$(CONTAINER_TOOL) build -f build/broker/Dockerfile -t ${BROKER_IMAGE} .

.PHONY: build-proxy-image
build-proxy-image:
	$(CONTAINER_TOOL) build -f build/proxy/Dockerfile -t ${PROXY_IMAGE} .

.PHONY: build-msc-image
build-msc-image:
	$(CONTAINER_TOOL) build -f build/msc/Dockerfile -t ${MSC_IMAGE} .

.PHONY: build-and-push-all-images
build-and-push-all-images: build-broker-image build-proxy-image build-msc-image
	$(CONTAINER_TOOL) push ${PROXY_IMAGE}
	$(CONTAINER_TOOL) push ${MSC_IMAGE}
	$(CONTAINER_TOOL) push ${BROKER_IMAGE}

.PHONY: build-test-images
build-test-images:
	$(CONTAINER_TOOL) build -f tests/mock/app/Dockerfile -t "edgewize/app-mock:latest" .
	$(CONTAINER_TOOL) build -f tests/mock/model/Dockerfile -t "edgewize/model-mock:latest" .

.PHONY: docker-buildx-test-images
docker-buildx-test-images:
	$(CONTAINER_TOOL) buildx build --builder mybuilder -t "harbor.dev.thingsdao.com/test/app:latest" --platform linux/amd64,linux/arm64  -f tests/mock/app/Dockerfile.buildx --push .
	$(CONTAINER_TOOL) buildx build --builder mybuilder -t "harbor.dev.thingsdao.com/test/model:latest" --platform linux/amd64,linux/arm64  -f tests/mock/model/Dockerfile.buildx --push .


.PHONY: docker-buildx-msc-image
docker-buildx-msc-image: ## Build and push docker image for the msc for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/msc/Dockerfile > Dockerfile.msc-cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${MSC_IMAGE} -f Dockerfile.msc-cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.msc-cross

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
build-muti-architecture-images: docker-buildx-msc-image docker-buildx-proxy-image docker-buildx-broker-image

# Generate manifests e.g. CRD, RBAC, etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="github.com/edgewize-io/edgewize/pkg/apis/${MANIFESTS}" output:crd:artifacts:config=config/crd

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt"  paths="./..."

##@ Tools

# Download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1  ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
