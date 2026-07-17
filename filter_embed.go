package filterscripts

import (
	"embed"
	"fmt"
)

//go:embed filter.lua
var scriptFiles embed.FS

func ListFiles(_ string) ([]string, error) {
	return []string{"filter.lua"}, nil
}

func ReadFile(file string) (string, error) {
	content, err := scriptFiles.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded lua script %s: %w", file, err)
	}

	return string(content), nil
}
