#!/bin/bash
set -eo pipefail

go build -C cmd -o cmd

set -a      # turn on automatic exporting
. .env  # source test.env
set +a      # turn off automatic exporting

./cmd/cmd