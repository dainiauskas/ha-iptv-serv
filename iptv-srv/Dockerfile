ARG BUILD_FROM=ghcr.io/hassio-addons/base:14.2.1

# Stage 1: build the binary
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .

RUN go mod init iptv-srv && \
    go build -ldflags="-w -s" -o iptv-srv main.go

# Stage 2: Hass.io add-on image
FROM $BUILD_FROM

COPY --from=builder /app/iptv-srv /usr/bin/iptv-srv

# Add and prepare run script
COPY run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
