#!/usr/bin/env bashio

# Pranešame sistemos žurnale apie paleidimą
bashio::log.info "Starting IPTV proxy server..."

# Paleidžiame mūsų sukompiliuotą programą
exec /usr/bin/iptv-srv
