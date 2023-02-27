package main

import (
	"embed"
	"path/filepath"
)

//go:embed resources/vendor
var vendor embed.FS

func getVendoredFile(lib, file string) ([]byte, error) {
	return vendor.ReadFile(filepath.Join("resources/vendor/", lib, file))
}
