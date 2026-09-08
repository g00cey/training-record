# syntax=docker/dockerfile:1

# --- build -----------------------------------------------------------------
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# modernc.org/sqlite is pure Go -> CGO disabled -> static binary
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /out/server ./cmd/server
# Pre-create the data dir owned by the distroless `nonroot` user (UID 65532).
# An empty named volume mounted here inherits this ownership, so the backend
# can create /data/training.db on first boot without an init container.
RUN mkdir -p /data && chown 65532:65532 /data

# --- dev (hot-reload via `go run`, source bind-mounted by compose.dev) ----
FROM golang:1.25 AS dev
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
EXPOSE 8080
CMD ["go", "run", "./cmd/server"]

# --- prod ----------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS prod
WORKDIR /app
COPY --from=build /out/server /app/server
# migrations are also embedded in the binary; copied for transparency/debug
COPY --from=build /src/migrations /app/migrations
# Carry over the nonroot-owned /data mountpoint so a fresh named volume is
# writable by the backend (UID 65532) without a host-side chown.
COPY --from=build --chown=65532:65532 /data /data
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/server"]
