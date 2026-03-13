#!/usr/bin/env bashio

# Log startup
bashio::log.info "Starting IPTV proxy server..."

# Run the compiled binary
exec /usr/bin/iptv-srv
