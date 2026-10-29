#!/bin/sh
set -eu
target="$1"
curl -fsS http://localhost:8080/healthz -o "$target"
