//go:build linux || darwin

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
// at every step. It also prevents blocking on special files (FIFOs, devices)
// and rejects non-regular files.
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
		} else {
			// O_NONBLOCK prevents hanging on FIFOs and devices
			flags |= unix.O_NONBLOCK
		}

		fd, err := unix.Openat(currentFd, part, flags, 0)
		if currentFd != rootFd {
			unix.Close(currentFd)
		}
		if err != nil {
			if err == unix.ENOENT {
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

	// Verify the final file descriptor points to a regular file
	var stat unix.Stat_t
	if err := unix.Fstat(currentFd, &stat); err != nil {
		unix.Close(currentFd)
		return nil, fmt.Errorf("fstat failed: %w", err)
	}

	mode := stat.Mode & unix.S_IFMT
	if mode != unix.S_IFREG {
		unix.Close(currentFd)
		return nil, fmt.Errorf("refusing to read non-regular file (mode %o)", stat.Mode)
	}

	file := os.NewFile(uintptr(currentFd), target)
	if file == nil {
		unix.Close(currentFd)
		return nil, fmt.Errorf("invalid fd")
	}
	defer file.Close()

	return io.ReadAll(file)
}
