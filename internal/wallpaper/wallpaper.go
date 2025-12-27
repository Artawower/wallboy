// Package wallpaper handles setting the desktop wallpaper.
package wallpaper

import (
	"path/filepath"

	"github.com/darkawower/wallboy/internal/platform"
)

// Setter is the interface for setting wallpapers.
type Setter interface {
	// Set sets the wallpaper to the specified path on the current desktop.
	Set(path string) error

	// Get returns the current wallpaper path (if available).
	Get() (string, error)
}

// platformSetter wraps platform.WallpaperService to implement Setter.
type platformSetter struct {
	svc platform.WallpaperService
}

// NewSetter creates a new wallpaper setter for the current platform.
func NewSetter() Setter {
	return &platformSetter{
		svc: platform.Current().Wallpaper(),
	}
}

// Set sets the wallpaper to the specified path.
func (s *platformSetter) Set(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return s.svc.Set(absPath)
}

// Get returns the current wallpaper path.
func (s *platformSetter) Get() (string, error) {
	return s.svc.Get()
}

// OpenInFinder opens the file in the system file manager.
func OpenInFinder(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return platform.Current().FileManager().Reveal(absPath)
}

// OpenImage opens the image in the default viewer.
func OpenImage(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return platform.Current().FileManager().Open(absPath)
}
