FROM golang:1.25-alpine AS build

# Cache module dependencies
COPY go.mod /trek/go.mod
COPY go.sum /trek/go.sum
WORKDIR /trek
RUN go mod download

# Add source and build the actual component
COPY ./cmd /trek/cmd
COPY ./internal /trek/internal
COPY ./main.go /trek
RUN go build -o dist/bin/trek .


FROM alpine:latest AS release
# Install PostgreSQL 18 client (must match embedded postgres version)
RUN apk add postgresql18-client bash
COPY --from=build /trek/dist/bin/trek /usr/local/bin/trek

RUN adduser -D -u 1001 unprivileged
USER unprivileged

WORKDIR /data
