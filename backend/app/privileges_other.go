//go:build !linux

package app

// DropRoot only does something on Linux, where the container runs.
func DropRoot(dataDir string) {}
