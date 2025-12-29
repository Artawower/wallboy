// Package ui provides terminal UI utilities for wallboy.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Colors for terminal output
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"

	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
)

// Symbols for different message types
const (
	SymbolSuccess = "✔"
	SymbolError   = "✖"
	SymbolWarning = "⚠"
	SymbolInfo    = "ℹ"
	SymbolArrow   = "→"
	SymbolBullet  = "•"
)

// Output wraps an io.Writer with UI utilities.
type Output struct {
	w       io.Writer
	noColor bool
	quiet   bool
	verbose bool
}

// NewOutput creates a new Output.
func NewOutput(w io.Writer) *Output {
	return &Output{w: w}
}

// DefaultOutput creates an Output for stdout.
func DefaultOutput() *Output {
	return NewOutput(os.Stdout)
}

// SetNoColor disables colors.
func (o *Output) SetNoColor(noColor bool) {
	o.noColor = noColor
}

// SetQuiet enables quiet mode (only errors).
func (o *Output) SetQuiet(quiet bool) {
	o.quiet = quiet
}

// SetVerbose enables verbose mode.
func (o *Output) SetVerbose(verbose bool) {
	o.verbose = verbose
}

// color applies color if enabled.
func (o *Output) color(code, text string) string {
	if o.noColor {
		return text
	}
	return code + text + Reset
}

// Success prints a success message.
func (o *Output) Success(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Green, SymbolSuccess), msg)
}

// Error prints an error message.
func (o *Output) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Red, SymbolError), msg)
}

// ErrorWithHint prints an error message with a hint.
func (o *Output) ErrorWithHint(err, hint string) {
	fmt.Fprintf(o.w, "%s %s\n", o.color(Red, SymbolError), err)
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, "Hint:"), hint)
}

// Warning prints a warning message.
func (o *Output) Warning(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Yellow, SymbolWarning), msg)
}

// Info prints an info message.
func (o *Output) Info(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Blue, SymbolInfo), msg)
}

// Print prints a plain message.
func (o *Output) Print(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, format+"\n", args...)
}

// Printf prints without newline.
func (o *Output) Printf(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, format, args...)
}

// Debug prints a debug message (only in verbose mode).
func (o *Output) Debug(format string, args ...interface{}) {
	if !o.verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Gray, "[DEBUG]"), msg)
}

// Field prints a labeled field.
func (o *Output) Field(label, value string) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, label+":"), value)
}

// FieldColored prints a labeled field with colored value.
func (o *Output) FieldColored(label, value, color string) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, label+":"), o.color(color, value))
}

// Table prints a simple table.
func (o *Output) Table(headers []string, rows [][]string) {
	if o.quiet {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	headerLine := ""
	for i, h := range headers {
		headerLine += fmt.Sprintf("%-*s  ", widths[i], h)
	}
	fmt.Fprintln(o.w, o.color(Bold, strings.TrimSpace(headerLine)))

	// Print separator
	sepLine := ""
	for _, w := range widths {
		sepLine += strings.Repeat("-", w) + "  "
	}
	fmt.Fprintln(o.w, o.color(Gray, strings.TrimSpace(sepLine)))

	// Print rows
	for _, row := range rows {
		rowLine := ""
		for i, cell := range row {
			if i < len(widths) {
				rowLine += fmt.Sprintf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Fprintln(o.w, strings.TrimSpace(rowLine))
	}
}

// Spinner represents a CLI spinner.
type Spinner struct {
	out      *Output
	message  string
	frames   []string
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewSpinner creates a new spinner.
func NewSpinner(out *Output, message string) *Spinner {
	return &Spinner{
		out:      out,
		message:  message,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start starts the spinner.
func (s *Spinner) Start() {
	if s.out.quiet {
		return
	}

	go func() {
		defer close(s.done)
		i := 0
		for {
			select {
			case <-s.stop:
				// Clear the line
				fmt.Fprintf(s.out.w, "\r%s\r", strings.Repeat(" ", len(s.message)+4))
				return
			default:
				frame := s.frames[i%len(s.frames)]
				fmt.Fprintf(s.out.w, "\r%s %s", s.out.color(Cyan, frame), s.message)
				time.Sleep(s.interval)
				i++
			}
		}
	}()
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	if s.out.quiet {
		return
	}
	close(s.stop)
	<-s.done
}

// Progress represents a progress bar.
type Progress struct {
	out     *Output
	message string
	current int
	total   int
}

// NewProgress creates a new progress indicator.
func NewProgress(out *Output, message string, total int) *Progress {
	return &Progress{
		out:     out,
		message: message,
		total:   total,
	}
}

// Update updates the progress.
func (p *Progress) Update(current int) {
	if p.out.quiet {
		return
	}
	p.current = current
	fmt.Fprintf(p.out.w, "\r%s (%d/%d)", p.message, current, p.total)
}

// Done completes the progress.
func (p *Progress) Done() {
	if p.out.quiet {
		return
	}
	fmt.Fprintf(p.out.w, "\r%s\r", strings.Repeat(" ", len(p.message)+20))
}

// WallpaperInfo prints formatted wallpaper information.
func (o *Output) WallpaperInfo(theme, source, path, query string, setAt time.Time) {
	o.Success("Wallpaper set")
	o.Field("Theme", theme)
	o.Field("Source", source)
	if query != "" {
		o.Field("Query", query)
	}
	o.Field("File", path)
	if !setAt.IsZero() {
		o.Field("Set at", setAt.Format("2006-01-02 15:04:05"))
	}
}

// ColorSwatch prints a color swatch.
func (o *Output) ColorSwatch(hex string) {
	if o.quiet {
		return
	}
	// Use the hex color as an ANSI 24-bit color for the block
	// Parse hex color
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)

	// Print a colored block followed by the hex code
	block := fmt.Sprintf("\033[48;2;%d;%d;%dm  \033[0m", r, g, b)
	fmt.Fprintf(o.w, "%s %s\n", block, hex)
}
