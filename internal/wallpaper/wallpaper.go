package wallpaper

import (
	"path/filepath"

	"github.com/Artawower/wallboy/internal/platform"
)

type Setter interface {
	Set(path string) error
	Get() (string, error)
}

type platformSetter struct {
	svc platform.WallpaperService
}

func NewSetter() Setter {
	return &platformSetter{
		svc: platform.Current().Wallpaper(),
	}
}

func (s *platformSetter) Set(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return s.svc.Set(absPath)
}

func (s *platformSetter) Get() (string, error) {
	return s.svc.Get()
}

func OpenInFinder(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return platform.Current().FileManager().Reveal(absPath)
}

func OpenImage(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return platform.Current().FileManager().Open(absPath)
}
