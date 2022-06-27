#!/usr/bin/env bash

set -euo pipefail

go mod tidy
go mod vendor

git apply hack/patches/*.patch
