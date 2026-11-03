//go:build !linux && !darwin

package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// secureReadFile opens a file with O_NOFOLLOW to prevent TOCTOU symlink attacks.
func secureReadFile(root, target string) ([]byte, error) {
	f, err := os.OpenFile(target, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if os.IsNotExist(err) {
			if info, lerr := os.Lstat(target); lerr == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("refusing to read dangling symlink at %s", target)
				}
			}
			return []byte{}, nil
		}
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("refusing to follow symlink at %s", target)
		}
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
