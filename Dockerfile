# Dockerfile.distroless
FROM golang:1.25-alpine as base

ENV APP_HOME /go/src/blog
RUN mkdir -p "$APP_HOME"

WORKDIR "$APP_HOME"

COPY . .

RUN go mod download
RUN go mod verify
RUN go mod tidy

ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETVARIANT
RUN printf "I'm building for TARGETPLATFORM=${TARGETPLATFORM}" \
    && printf ", TARGETARCH=${TARGETARCH}" \
    && printf ", TARGETVARIANT=${TARGETVARIANT} \n" \
    && printf "With uname -s : " && uname -s \
    && printf "and  uname -m : " && uname -m

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o /main .

FROM gcr.io/distroless/static-debian11 as production

COPY --from=base /main .
COPY --from=base /go/src/blog/static ./static
COPY --from=base /go/src/blog/templates ./templates
COPY --from=base /go/src/blog/themes ./themes
COPY --from=base /go/src/blog/css ./css

CMD ["./main", "--listen-addr", ":22222"]
