package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var _version string

var version string

func init() {
	if _version == "" {
		_version = "DEV"
	}
	version = strings.TrimSpace(_version)
}
