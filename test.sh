#!/bin/bash
set -eo pipefail

set -a      # turn on automatic exporting
. .env  # source test.env
set +a      # turn off automatic exporting

go test -v ./...