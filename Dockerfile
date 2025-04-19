# Use the official Golang image to create a build artifact.
FROM golang:1.24 as gobuilder

# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies.
COPY go.* ./
# RUN go mod download

# Copy local code to the container image.
COPY . ./

# Build the binary for the target architecture.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -v rtl_433_prometheus.go

# Use a minimal base image for the final stage.
FROM debian:bullseye-slim

# Install runtime dependencies.
RUN apt-get update && apt-get install -y --no-install-recommends librtlsdr0 && apt-get clean && rm -rf /var/lib/apt/lists/*

# Copy the built binary and rtl_433 tool from the builder stage.
WORKDIR /
COPY --from=gobuilder /app/rtl_433_prometheus /
COPY --from=hertzg/rtl_433:latest /usr/local/bin/rtl_433 /
RUN chmod +x /rtl_433 /rtl_433_prometheus

# Expose the Prometheus metrics port.
EXPOSE 9550

# Set the entrypoint and default command.
ENTRYPOINT ["/rtl_433_prometheus"]
CMD ["--subprocess", "/rtl_433 -F json -M newmodel"]

