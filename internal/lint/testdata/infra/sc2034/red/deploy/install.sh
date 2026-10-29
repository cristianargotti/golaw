#!/bin/sh
set -eu
PREVIOUS_TAG="$(cat /srv/hub/tag)"
printf 'installing %s\n' "$1"
