package bokchoy

// Ported from Goji's middleware, source:
// https://github.com/zenazn/goji/tree/master/web/middleware

import (
	"fmt"
	"io"
	"os"
)

var (
	// Normal colors
	ColorBlack   = []byte{'\033', '[', '3', '0', 'm'}
	ColorRed     = []byte{'\033', '[', '3', '1', 'm'}
	ColorGreen   = []byte{'\033', '[', '3', '2', 'm'}
	ColorYellow  = []byte{'\033', '[', '3', '3', 'm'}
	ColorBlue    = []byte{'\033', '[', '3', '4', 'm'}
	ColorMagenta = []byte{'\033', '[', '3', '5', 'm'}
	ColorCyan    = []byte{'\033', '[', '3', '6', 'm'}
	ColorWhite   = []byte{'\033', '[', '3', '7', 'm'}

	// Bright colors
	ColorBrightBlack   = []byte{'\033', '[', '3', '0', ';', '1', 'm'}
	ColorBrightRed     = []byte{'\033', '[', '3', '1', ';', '1', 'm'}
	ColorBrightGreen   = []byte{'\033', '[', '3', '2', ';', '1', 'm'}
	ColorBrightYellow  = []byte{'\033', '[', '3', '3', ';', '1', 'm'}
	ColorBrightBlue    = []byte{'\033', '[', '3', '4', ';', '1', 'm'}
	ColorBrightMagenta = []byte{'\033', '[', '3', '5', ';', '1', 'm'}
	ColorBrightCyan    = []byte{'\033', '[', '3', '6', ';', '1', 'm'}
	ColorBrightWhite   = []byte{'\033', '[', '3', '7', ';', '1', 'm'}

	ColorReset = []byte{'\033', '[', '0', 'm'}
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

// ColorWrite writes an output to stdout.
// nolint: errcheck
func ColorWrite(w io.Writer, useColor bool, color []byte, s string, args ...interface{}) {
	if isTTY && useColor {
		w.Write(color)
	}

	fmt.Fprintf(w, s, args...)
	if isTTY && useColor {
		w.Write(ColorReset)
	}
}
