# Build the manager binary
FROM golang:1.24 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY . .
RUN go mod tidy
RUN make build-controller-manager

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details

#FROM gcr.io/distroless/static:nonroot
FROM golang:1.24
WORKDIR /
COPY --from=builder /workspace/controller-manager .
USER 65532:65532

ENTRYPOINT ["/controller-manager"]