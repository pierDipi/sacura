#!/usr/bin/env bash

go mod tidy
go mod vendor

git apply hack/patches/*.patch
