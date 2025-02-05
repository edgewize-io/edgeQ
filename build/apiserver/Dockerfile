FROM golang:1.24 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY . .
RUN go mod tidy
RUN make build-apiserver

FROM golang:1.24
WORKDIR /
COPY --from=builder /workspace/apiserver .
USER 65532:65532

ENTRYPOINT ["/apiserver"]