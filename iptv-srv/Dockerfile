ARG BUILD_FROM=ghcr.io/hassio-addons/base:14.2.1

# 1 etapas: Programos kompiliavimas naudojant minimalų Golang atvaizdą
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .

# Sukuriame go.mod kad užtikrintume tiesioginį kompiliavimą
RUN go mod init iptv-srv && \
    go build -ldflags="-w -s" -o iptv-srv main.go

# 2 etapas: Hass.io add-on atvaizdo formavimas
FROM $BUILD_FROM

# Nukopijuojame sukompiliuotą programą iš builder etapo
COPY --from=builder /app/iptv-srv /usr/bin/iptv-srv

# Pridedame ir paruošiame paleidimo skriptą
COPY run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
