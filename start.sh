#!/usr/bin/env bash
podman start --filter status=exited postgres pgadmin
tailscale serve --bg --https=8443 http://localhost:8000 && rm -f ./fmk-api && go build && ./fmk-api
