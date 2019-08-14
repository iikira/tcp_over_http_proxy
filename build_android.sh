#!/bin/sh

export CC=aarch64-linux-android-gcc GOOS=android GOARCH=arm64 CGO_ENABLED=1
go build -ldflags '-s -w'
