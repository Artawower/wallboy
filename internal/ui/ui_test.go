package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOutput(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	require.NotNil(t, o)
	assert.Equal(t, &buf, o.w)
}

func TestDefaultOutput(t *testing.T) {
	o := DefaultOutput()
	require.NotNil(t, o)
}

func TestOutput_SetNoColor(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.SetNoColor(true)
	assert.True(t, o.noColor)

	o.SetNoColor(false)
	assert.False(t, o.noColor)
}

func TestOutput_SetQuiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.SetQuiet(true)
	assert.True(t, o.quiet)

	o.SetQuiet(false)
	assert.False(t, o.quiet)
}

func TestOutput_SetVerbose(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.SetVerbose(true)
	assert.True(t, o.verbose)

	o.SetVerbose(false)
	assert.False(t, o.verbose)
}

func TestOutput_color(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	t.Run("with color", func(t *testing.T) {
		result := o.color(Green, "test")
		assert.Contains(t, result, Green)
		assert.Contains(t, result, Reset)
		assert.Contains(t, result, "test")
	})

	t.Run("without color", func(t *testing.T) {
		o.SetNoColor(true)
		result := o.color(Green, "test")
		assert.Equal(t, "test", result)
		assert.NotContains(t, result, Green)
	})
}

func TestOutput_Success(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Success("Success message %s", "arg")
	assert.Contains(t, buf.String(), SymbolSuccess)
	assert.Contains(t, buf.String(), "Success message arg")
}

func TestOutput_Success_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Success("Success message")
	assert.Empty(t, buf.String())
}

func TestOutput_Error(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Error("Error message %s", "arg")
	assert.Contains(t, buf.String(), SymbolError)
	assert.Contains(t, buf.String(), "Error message arg")
}

func TestOutput_Error_NotQuiet(t *testing.T) {
	// Error should show even in quiet mode
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Error("Error message")
	assert.Contains(t, buf.String(), "Error message")
}

func TestOutput_ErrorWithHint(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.ErrorWithHint("Something went wrong", "Try this instead")
	assert.Contains(t, buf.String(), SymbolError)
	assert.Contains(t, buf.String(), "Something went wrong")
	assert.Contains(t, buf.String(), "Hint:")
	assert.Contains(t, buf.String(), "Try this instead")
}

func TestOutput_Warning(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Warning("Warning message %s", "arg")
	assert.Contains(t, buf.String(), SymbolWarning)
	assert.Contains(t, buf.String(), "Warning message arg")
}

func TestOutput_Warning_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Warning("Warning message")
	assert.Empty(t, buf.String())
}

func TestOutput_Info(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Info("Info message %s", "arg")
	assert.Contains(t, buf.String(), SymbolInfo)
	assert.Contains(t, buf.String(), "Info message arg")
}

func TestOutput_Info_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Info("Info message")
	assert.Empty(t, buf.String())
}

func TestOutput_Print(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Print("Plain message %s", "arg")
	assert.Contains(t, buf.String(), "Plain message arg")
	assert.True(t, strings.HasSuffix(buf.String(), "\n"))
}

func TestOutput_Print_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Print("Plain message")
	assert.Empty(t, buf.String())
}

func TestOutput_Printf(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Printf("No newline %s", "arg")
	assert.Equal(t, "No newline arg", buf.String())
	assert.False(t, strings.HasSuffix(buf.String(), "\n"))
}

func TestOutput_Printf_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Printf("No newline")
	assert.Empty(t, buf.String())
}

func TestOutput_Debug(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	t.Run("verbose mode", func(t *testing.T) {
		buf.Reset()
		o.SetVerbose(true)
		o.Debug("Debug message %s", "arg")
		assert.Contains(t, buf.String(), "[DEBUG]")
		assert.Contains(t, buf.String(), "Debug message arg")
	})

	t.Run("non-verbose mode", func(t *testing.T) {
		buf.Reset()
		o.SetVerbose(false)
		o.Debug("Debug message")
		assert.Empty(t, buf.String())
	})
}

func TestOutput_Field(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.Field("Label", "Value")
	assert.Contains(t, buf.String(), "Label:")
	assert.Contains(t, buf.String(), "Value")
}

func TestOutput_Field_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Field("Label", "Value")
	assert.Empty(t, buf.String())
}

func TestOutput_FieldColored(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.FieldColored("Label", "Value", Green)
	assert.Contains(t, buf.String(), "Label:")
	assert.Contains(t, buf.String(), "Value")
}

func TestOutput_FieldColored_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.FieldColored("Label", "Value", Green)
	assert.Empty(t, buf.String())
}

func TestOutput_Table(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"Row1", "Val1"},
		{"Row2", "Val2"},
	}

	o.Table(headers, rows)

	output := buf.String()
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Value")
	assert.Contains(t, output, "Row1")
	assert.Contains(t, output, "Val1")
	assert.Contains(t, output, "Row2")
	assert.Contains(t, output, "---")
}

func TestOutput_Table_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.Table([]string{"Header"}, [][]string{{"Row"}})
	assert.Empty(t, buf.String())
}

func TestOutput_WallpaperInfo(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	now := time.Now()
	o.WallpaperInfo("light", "local", "/path/to/image.jpg", "", now)

	output := buf.String()
	assert.Contains(t, output, "Wallpaper set")
	assert.Contains(t, output, "Theme:")
	assert.Contains(t, output, "light")
	assert.Contains(t, output, "Source:")
	assert.Contains(t, output, "local")
	assert.Contains(t, output, "File:")
	assert.Contains(t, output, "/path/to/image.jpg")
	assert.Contains(t, output, "Set at:")
}

func TestOutput_WallpaperInfo_NoTime(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.WallpaperInfo("dark", "remote", "/path/to/image.jpg", "", time.Time{})

	output := buf.String()
	assert.Contains(t, output, "dark")
	assert.NotContains(t, output, "Set at:")
}

func TestOutput_ColorSwatch(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	o.ColorSwatch("#ff0000")

	output := buf.String()
	assert.Contains(t, output, "#ff0000")
	// Contains ANSI escape codes for color
	assert.Contains(t, output, "\033[48;2;255;0;0m")
}

func TestOutput_ColorSwatch_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	o.ColorSwatch("#ff0000")
	assert.Empty(t, buf.String())
}

func TestNewSpinner(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	s := NewSpinner(o, "Loading...")
	require.NotNil(t, s)
	assert.Equal(t, "Loading...", s.message)
	assert.NotNil(t, s.frames)
	assert.NotNil(t, s.stop)
	assert.NotNil(t, s.done)
}

func TestSpinner_StartStop(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	s := NewSpinner(o, "Test")
	s.Start()

	// Give the spinner time to render at least once
	time.Sleep(100 * time.Millisecond)

	s.Stop()

	// Spinner should have written something
	// (The buffer may be cleared when stopped)
}

func TestSpinner_StartStop_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	s := NewSpinner(o, "Test")
	s.Start()
	s.Stop()

	// In quiet mode, nothing should be written
	assert.Empty(t, buf.String())
}

func TestNewProgress(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	p := NewProgress(o, "Downloading...", 100)
	require.NotNil(t, p)
	assert.Equal(t, "Downloading...", p.message)
	assert.Equal(t, 100, p.total)
	assert.Equal(t, 0, p.current)
}

func TestProgress_Update(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	p := NewProgress(o, "Progress", 100)
	p.Update(50)

	assert.Contains(t, buf.String(), "Progress")
	assert.Contains(t, buf.String(), "50/100")
}

func TestProgress_Update_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	p := NewProgress(o, "Progress", 100)
	p.Update(50)

	assert.Empty(t, buf.String())
}

func TestProgress_Done(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)

	p := NewProgress(o, "Progress", 100)
	p.Update(100)
	buf.Reset()
	p.Done()

	// Done should clear the line
	assert.Contains(t, buf.String(), "\r")
}

func TestProgress_Done_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewOutput(&buf)
	o.SetQuiet(true)

	p := NewProgress(o, "Progress", 100)
	p.Done()

	assert.Empty(t, buf.String())
}

func TestConstants(t *testing.T) {
	// Test that color constants are set
	assert.NotEmpty(t, Reset)
	assert.NotEmpty(t, Bold)
	assert.NotEmpty(t, Red)
	assert.NotEmpty(t, Green)
	assert.NotEmpty(t, Yellow)
	assert.NotEmpty(t, Blue)
	assert.NotEmpty(t, Magenta)
	assert.NotEmpty(t, Cyan)
	assert.NotEmpty(t, White)
	assert.NotEmpty(t, Gray)

	// Test symbol constants
	assert.NotEmpty(t, SymbolSuccess)
	assert.NotEmpty(t, SymbolError)
	assert.NotEmpty(t, SymbolWarning)
	assert.NotEmpty(t, SymbolInfo)
	assert.NotEmpty(t, SymbolArrow)
	assert.NotEmpty(t, SymbolBullet)
}
