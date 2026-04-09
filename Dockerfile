# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGET=api
ARG VERSION=dev

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/app ./cmd/${TARGET}

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=builder /out/app /usr/local/bin/app

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/app"]
