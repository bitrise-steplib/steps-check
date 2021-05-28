#!/bin/env bash
set -ex

STEP_DIR="$(dirname "$0")"
if [[ ! -z "$step_dir" ]]; then
    STEP_DIR=$step_dir
fi

if [[ -z "$workflow"  ]]; then
    exit 1
fi

STEP_DIR=$STEP_DIR bitrise run $workflow --config "$(dirname "$0")"/workflows.yml
