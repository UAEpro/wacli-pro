package config

import (
	"os"
	"path/filepath"
)

func DefaultStoreDir() string {
	if dir := os.Getenv("WACLI_PRO_STORE_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".wacli-pro"
	}
	return filepath.Join(home, ".wacli-pro")
}
