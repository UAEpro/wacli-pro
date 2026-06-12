FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -tags sqlite_fts5 -ldflags "-s -w" -o /out/wacli-pro ./cmd/wacli-pro

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/wacli-pro /usr/local/bin/wacli-pro

# All state (session + message DB) lives here; mount a volume to persist it.
ENV WACLI_PRO_STORE_DIR=/data
VOLUME /data

ENTRYPOINT ["wacli-pro"]
CMD ["--help"]
