//go:build !windows

package config

import "os"

// RestrictDirToCurrentUser enforces 0700 owner-only permissions on POSIX platforms.
func RestrictDirToCurrentUser(path string) error {
	return os.Chmod(path, 0o700) // #nosec G302 -- directory requires owner execute bit (0700) for traversal
}
