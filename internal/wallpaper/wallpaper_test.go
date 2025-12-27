package wallpaper

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSetter(t *testing.T) {
	setter := NewSetter()
	require.NotNil(t, setter)

	// Verify we get the platformSetter wrapper
	_, ok := setter.(*platformSetter)
	assert.True(t, ok, "expected platformSetter wrapper")
}

func TestSetter_Get(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping test on non-darwin platform")
	}

	setter := NewSetter()

	// Test Get (should work on macOS)
	path, err := setter.Get()
	// May or may not error depending on permissions
	if err == nil {
		assert.NotEmpty(t, path)
		t.Logf("Current wallpaper: %s", path)
	}
}

func TestOpenInFinder(t *testing.T) {
	// We can't really test this without side effects
	// Just verify the function exists and handles paths

	t.Run("absolute path handling", func(t *testing.T) {
		// This will attempt to open a file that doesn't exist
		// which should fail, but we verify the path handling works
		err := OpenInFinder("/nonexistent/path/file.jpg")
		// Expected to fail (file doesn't exist or command fails)
		// but should not panic
		_ = err
	})
}

func TestOpenImage(t *testing.T) {
	// Similar to OpenInFinder, we can't test without side effects
	t.Run("absolute path handling", func(t *testing.T) {
		err := OpenImage("/nonexistent/path/file.jpg")
		// Expected to fail but should not panic
		_ = err
	})
}

// Test the Setter interface is properly implemented
func TestSetterInterface(t *testing.T) {
	var _ Setter = &platformSetter{}
}
