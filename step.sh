#!/bin/env bash
set -ex

STEP_DIR="$(dirname "$0")"
if [[ ! -z "$step_dir" ]]; then
    STEP_DIR=$step_dir
fi

STEP_DIR=$STEP_DIR bitrise run lint --config "$(dirname "$0")"/lint.yml