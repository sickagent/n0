FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY proto/gen/go proto/gen/go
COPY example/plugin example/plugin
RUN --mount=type=cache,target=/go/pkg/mod \
    cd example/plugin && GOWORK=off CGO_ENABLED=0 go build -o /example-plugin .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /example-plugin /example-plugin
USER nonroot:nonroot
EXPOSE 50051
ENTRYPOINT ["/example-plugin"]
