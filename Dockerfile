# Stage 1: Build
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod main.go Makefile ./
RUN apk add --no-cache make
RUN make build

# Stage 2: Minimal runtime
FROM scratch
COPY --from=builder /app/bin/agent /agent
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/agent"]
