# Build the manager binary
FROM golang:1.24 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY . .
RUN go mod tidy
RUN make build-broker

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/broker .
USER 65532:65532

ENTRYPOINT ["/broker"]
