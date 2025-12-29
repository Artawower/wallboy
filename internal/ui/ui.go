package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

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

const (
	SymbolSuccess = "✔"
	SymbolError   = "✖"
	SymbolWarning = "⚠"
	SymbolInfo    = "ℹ"
	SymbolArrow   = "→"
	SymbolBullet  = "•"
)

type Output struct {
	w       io.Writer
	noColor bool
	quiet   bool
	verbose bool
}

func NewOutput(w io.Writer) *Output {
	return &Output{w: w}
}

func DefaultOutput() *Output {
	return NewOutput(os.Stdout)
}

func (o *Output) SetNoColor(noColor bool) {
	o.noColor = noColor
}

func (o *Output) SetQuiet(quiet bool) {
	o.quiet = quiet
}

func (o *Output) SetVerbose(verbose bool) {
	o.verbose = verbose
}

func (o *Output) color(code, text string) string {
	if o.noColor {
		return text
	}
	return code + text + Reset
}

func (o *Output) Success(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Green, SymbolSuccess), msg)
}

func (o *Output) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Red, SymbolError), msg)
}

func (o *Output) ErrorWithHint(err, hint string) {
	fmt.Fprintf(o.w, "%s %s\n", o.color(Red, SymbolError), err)
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, "Hint:"), hint)
}

func (o *Output) Warning(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Yellow, SymbolWarning), msg)
}

func (o *Output) Info(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Blue, SymbolInfo), msg)
}

func (o *Output) Print(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, format+"\n", args...)
}

func (o *Output) Printf(format string, args ...interface{}) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, format, args...)
}

func (o *Output) Debug(format string, args ...interface{}) {
	if !o.verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.w, "%s %s\n", o.color(Gray, "[DEBUG]"), msg)
}

func (o *Output) Field(label, value string) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, label+":"), value)
}

func (o *Output) FieldColored(label, value, color string) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.w, "  %s %s\n", o.color(Gray, label+":"), o.color(color, value))
}

func (o *Output) Table(headers []string, rows [][]string) {
	if o.quiet {
		return
	}

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

	headerLine := ""
	for i, h := range headers {
		headerLine += fmt.Sprintf("%-*s  ", widths[i], h)
	}
	fmt.Fprintln(o.w, o.color(Bold, strings.TrimSpace(headerLine)))

	sepLine := ""
	for _, w := range widths {
		sepLine += strings.Repeat("-", w) + "  "
	}
	fmt.Fprintln(o.w, o.color(Gray, strings.TrimSpace(sepLine)))

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

type Spinner struct {
	out      *Output
	message  string
	frames   []string
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

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

func (s *Spinner) Stop() {
	if s.out.quiet {
		return
	}
	close(s.stop)
	<-s.done
}

type Progress struct {
	out     *Output
	message string
	current int
	total   int
}

func NewProgress(out *Output, message string, total int) *Progress {
	return &Progress{
		out:     out,
		message: message,
		total:   total,
	}
}

func (p *Progress) Update(current int) {
	if p.out.quiet {
		return
	}
	p.current = current
	fmt.Fprintf(p.out.w, "\r%s (%d/%d)", p.message, current, p.total)
}

func (p *Progress) Done() {
	if p.out.quiet {
		return
	}
	fmt.Fprintf(p.out.w, "\r%s\r", strings.Repeat(" ", len(p.message)+20))
}

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

func (o *Output) ColorSwatch(hex string) {
	if o.quiet {
		return
	}
	var r, g, b int
	_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)

	block := fmt.Sprintf("\033[48;2;%d;%d;%dm  \033[0m", r, g, b)
	fmt.Fprintf(o.w, "%s %s\n", block, hex)
}
