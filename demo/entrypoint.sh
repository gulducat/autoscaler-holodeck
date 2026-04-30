#!/usr/bin/env bash

set -xeuo pipefail

#APPDIR="$(realpath "$(dirname "${BASH_SOURCE[*]}")")"
APPDIR="${APPDIR:-$PWD}"
pushd "$APPDIR"

case $1 in
  bash)
    bash -i
    ;;
  autoscaler)
    nomad-autoscaler agent -config "${AUTOSCALER_AGENT_CONFIG:-./demo/autoscaler/agent.hcl}"
    ;;
  nodesim)
    nomad-nodesim -config ./demo/nodesim.hcl
    ;;
  observer|holodeck)
    eval "$1"
    ;;
  *) echo "unknown arg '$1'" ; exit 1 ;;
esac
