# stage 1: build static binary
FROM golang:1.24-alpine AS build

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -buildid= \
      -X dispatch/internal/version.Version=${VERSION} \
      -X dispatch/internal/version.Commit=${COMMIT} \
      -X dispatch/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/dispatch ./cmd/dispatch

# stage 2: minimal runtime
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/dispatch /dispatch

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

EXPOSE 18087

USER 65532:65532

ENTRYPOINT ["/dispatch"]
CMD ["--config", "/config/router.yaml"]
