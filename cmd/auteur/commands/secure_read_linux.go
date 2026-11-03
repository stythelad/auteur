//go:build linux

package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// secureReadFile safely opens a file by traversing each path component
// relative to the previously opened directory descriptor, refusing symlinks
// at every step. This completely eliminates TOCTOU races.
func secureReadFile(root, target string) ([]byte, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	realTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("target outside root")
	}

	// Open the root directory
	rootFd, err := unix.Open(realRoot, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, fmt.Errorf("open root: %w", err)
	}
	defer unix.Close(rootFd)

	currentFd := rootFd
	parts := strings.Split(rel, string(filepath.Separator))

	for i, part := range parts {
		isLast := i == len(parts)-1
		flags := unix.O_RDONLY | unix.O_NOFOLLOW
		if !isLast {
			flags |= unix.O_DIRECTORY
		}

		fd, err := unix.Openat(currentFd, part, flags, 0)
		if currentFd != rootFd {
			unix.Close(currentFd)
		}
		if err != nil {
			if os.IsNotExist(err) || err == unix.ENOENT {
				if isLast {
					return []byte{}, nil // New file
				}
				return nil, err
			}
			if err == unix.ELOOP {
				return nil, fmt.Errorf("refusing to follow symlink at %s", part)
			}
			return nil, err
		}
		currentFd = fd
	}

	file := os.NewFile(uintptr(currentFd), target)
	if file == nil {
		return nil, fmt.Errorf("invalid fd")
	}
	defer file.Close()

	return io.ReadAll(file)
}
