# Build stage — runs on host arch, cross-compiles via GOARCH
FROM --platform=$BUILDPLATFORM golang:1.26-alpine as base

ENV APP_HOME /go/src/blog
RUN mkdir -p "$APP_HOME"

WORKDIR "$APP_HOME"

COPY . .

RUN go mod download
RUN go mod verify
RUN go mod tidy

ARG TARGETARCH

# Version information for build-time injection
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
ARG GO_VERSION=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-X 'anshumanbiswas.com/blog/version.Version=${VERSION}' \
              -X 'anshumanbiswas.com/blog/version.GitCommit=${GIT_COMMIT}' \
              -X 'anshumanbiswas.com/blog/version.BuildTime=${BUILD_TIME}' \
              -X 'anshumanbiswas.com/blog/version.GoVersion=${GO_VERSION}'" \
    -o /main .

FROM alpine:latest as production

# Install PostgreSQL client for database backup/restore functionality
RUN apk add --no-cache postgresql-client ca-certificates

COPY --from=base /main .
COPY --from=base /go/src/blog/static ./static
COPY --from=base /go/src/blog/templates ./templates
COPY --from=base /go/src/blog/themes ./themes
COPY --from=base /go/src/blog/css ./css

CMD ["./main", "--listen-addr", ":22222"]
