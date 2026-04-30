#!/usr/bin/env bash

set -xeuo pipefail

cmd="$1"
shift

case $cmd in
  bash|nomad-autoscaler|nomad-nodesim|observer|holodeck)
    eval "$cmd $@"
    ;;
  *)
    echo "unknown cmd '$cmd'"
    exit 1
    ;;
esac
