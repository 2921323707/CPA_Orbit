//go:build windows

package main

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const startupRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func setStartOnLogin(executable string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, startupRegistryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open startup registry: %w", err)
	}
	defer key.Close()
	const valueName = "CPAOrbit"
	if !enabled {
		if err := key.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("remove startup entry: %w", err)
		}
		return nil
	}
	quoted := `"` + filepath.Clean(executable) + `"`
	if err := key.SetStringValue(valueName, quoted); err != nil {
		return fmt.Errorf("write startup entry: %w", err)
	}
	return nil
}
