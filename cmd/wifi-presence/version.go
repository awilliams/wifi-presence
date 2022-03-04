package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

func init() {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "DEV"
	}
}
