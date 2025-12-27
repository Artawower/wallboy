//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
)

// FileManagerService implements platform.FileManagerService for macOS.
type FileManagerService struct{}

// NewFileManagerService creates a new macOS file manager service.
func NewFileManagerService() *FileManagerService {
	return &FileManagerService{}
}

// Reveal opens Finder and highlights the specified file.
func (s *FileManagerService) Reveal(path string) error {
	cmd := exec.Command("open", "-R", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reveal in Finder: %w (output: %s)", err, string(output))
	}
	return nil
}

// Open opens the file with the default application.
func (s *FileManagerService) Open(path string) error {
	cmd := exec.Command("open", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open file: %w (output: %s)", err, string(output))
	}
	return nil
}
