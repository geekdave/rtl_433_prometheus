# syntax=docker/dockerfile:1.4

################################################################
# 1) Builder – compile Go binary for rtl_433_prometheus
################################################################
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

# These args/platforms let BuildKit cross‑compile automatically
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    CGO_ENABLED=0

WORKDIR /app
# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code and build
COPY . .
RUN go build -o /usr/local/bin/rtl_433_prometheus ./cmd/rtl_433_prometheus

################################################################
# 2) Runtime – install only the SDR library + rtl_433 binary
################################################################
FROM --platform=$TARGETPLATFORM debian:bookworm-slim

# Install RTL-SDR library and rtl_433 from the Debian repo
RUN apt-get update \
 && apt-get install -y librtlsdr0 rtl-433 \
 && rm -rf /var/lib/apt/lists/*

# Copy our Go-based exporter
COPY --from=builder /usr/local/bin/rtl_433_prometheus /usr/local/bin/

EXPOSE 9550
ENTRYPOINT ["/usr/local/bin/rtl_433_prometheus"]
CMD ["--subprocess", "/usr/bin/rtl_433", "-F", "json", "-M", "newmodel"]
