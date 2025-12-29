//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
)

type FileManagerService struct{}

func NewFileManagerService() *FileManagerService {
	return &FileManagerService{}
}

func (s *FileManagerService) Reveal(path string) error {
	cmd := exec.Command("open", "-R", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reveal in Finder: %w (output: %s)", err, string(output))
	}
	return nil
}

func (s *FileManagerService) Open(path string) error {
	cmd := exec.Command("open", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to open file: %w (output: %s)", err, string(output))
	}
	return nil
}
