# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.7

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src/backend

RUN apk add --no-cache ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S app \
	&& adduser -S -G app app

WORKDIR /app

COPY --from=build /out/server /app/server

USER app

EXPOSE 8080

ENTRYPOINT ["/app/server"]
