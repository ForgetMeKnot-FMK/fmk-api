#!/usr/bin/env bash
podman start --filter status=exited postgres pgadmin
rm -f ./fmk-api && go build && ./fmk-api

