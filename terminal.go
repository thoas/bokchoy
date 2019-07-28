package bokchoy

// Ported from Goji's middleware, source:
// https://github.com/zenazn/goji/tree/master/web/middleware

import (
	"bytes"
	"fmt"
	"os"
)

type Color []byte

var (
	// Normal colors
	ColorBlack   = Color{'\033', '[', '3', '0', 'm'}
	ColorRed     = Color{'\033', '[', '3', '1', 'm'}
	ColorGreen   = Color{'\033', '[', '3', '2', 'm'}
	ColorYellow  = Color{'\033', '[', '3', '3', 'm'}
	ColorBlue    = Color{'\033', '[', '3', '4', 'm'}
	ColorMagenta = Color{'\033', '[', '3', '5', 'm'}
	ColorCyan    = Color{'\033', '[', '3', '6', 'm'}
	ColorWhite   = Color{'\033', '[', '3', '7', 'm'}

	// Bright colors
	ColorBrightBlack   = Color{'\033', '[', '3', '0', ';', '1', 'm'}
	ColorBrightRed     = Color{'\033', '[', '3', '1', ';', '1', 'm'}
	ColorBrightGreen   = Color{'\033', '[', '3', '2', ';', '1', 'm'}
	ColorBrightYellow  = Color{'\033', '[', '3', '3', ';', '1', 'm'}
	ColorBrightBlue    = Color{'\033', '[', '3', '4', ';', '1', 'm'}
	ColorBrightMagenta = Color{'\033', '[', '3', '5', ';', '1', 'm'}
	ColorBrightCyan    = Color{'\033', '[', '3', '6', ';', '1', 'm'}
	ColorBrightWhite   = Color{'\033', '[', '3', '7', ';', '1', 'm'}

	ColorReset = Color{'\033', '[', '0', 'm'}
)

var isTTY bool

func init() {
	// This is sort of cheating: if stdout is a character device, we assume
	// that means it's a TTY. Unfortunately, there are many non-TTY
	// character devices, but fortunately stdout is rarely set to any of
	// them.
	//
	// We could solve this properly by pulling in a dependency on
	// code.google.com/p/go.crypto/ssh/terminal, for instance, but as a
	// heuristic for whether to print in color or in black-and-white, I'd
	// really rather not.
	fi, err := os.Stdout.Stat()
	if err == nil {
		m := os.ModeDevice | os.ModeCharDevice
		isTTY = fi.Mode()&m == m
	}
}

// ColorWriter is a bytes buffer with color.
type ColorWriter struct {
	*bytes.Buffer
	color Color
}

// WithColor returns a new ColorWriter with a new color.
func (c ColorWriter) WithColor(color Color) *ColorWriter {
	c.color = color
	return &c
}

// NewColorWriter initializes a new ColorWriter.
func NewColorWriter(color Color) *ColorWriter {
	return &ColorWriter{
		&bytes.Buffer{},
		color,
	}
}

// Write writes an output to stdout.
// nolint: errcheck
func (c *ColorWriter) Write(s string, args ...interface{}) {
	if isTTY && c.color != nil {
		c.Buffer.Write(c.color)
	}

	fmt.Fprintf(c.Buffer, s, args...)
	if isTTY && c.color != nil {
		c.Buffer.Write(ColorReset)
	}
}
