package main

import "os"

// isTerminal reports whether f is attached to a character device (a TTY).
// Used to decide whether ANSI colors should be emitted. This avoids pulling in
// a third-party dependency for such a small check.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
