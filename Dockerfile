# syntax=docker/dockerfile:1.4

################################################################
# 1) Builder – compile Go binary for rtl_433_prometheus
################################################################
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

# Enable cross‑compile args for buildx
ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    CGO_ENABLED=0

WORKDIR /app
# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the exporter
COPY . .
# Adjust path to where your main package resides. If main.go is at root:
RUN go build -o /usr/local/bin/rtl_433_prometheus .

################################################################
# 2) Runtime – install SDR library + rtl_433 binary
################################################################
FROM --platform=$TARGETPLATFORM debian:bookworm-slim

# Pull in RTL-SDR library and the rtl-433 executable from Debian
RUN apt-get update \
 && apt-get install -y librtlsdr0 rtl-433 \
 && rm -rf /var/lib/apt/lists/*

# Copy compiled exporter
COPY --from=builder /usr/local/bin/rtl_433_prometheus /usr/local/bin/

EXPOSE 9550
ENTRYPOINT ["/usr/local/bin/rtl_433_prometheus"]
CMD ["--subprocess", "/usr/bin/rtl_433", "-F", "json", "-M", "newmodel"]
