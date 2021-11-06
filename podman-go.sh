#!/bin/sh

set -eu

podman run --rm -v "$PWD:/usr/src/myapp:Z" -w /usr/src/myapp golang:buster "$@"
