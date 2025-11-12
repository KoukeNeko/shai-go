package cli

import (
	"fmt"
	"io"
)

// streamWriter writes streaming output to an io.Writer.
type streamWriter struct {
	out io.Writer
}

// NewStreamWriter builds a streamWriter for stdout/stderr.
func NewStreamWriter(out io.Writer) *streamWriter {
	return &streamWriter{out: out}
}

func (s *streamWriter) WriteChunk(text string) {
	if text == "" {
		return
	}
	fmt.Fprintln(s.out, text)
}

func (s *streamWriter) Done() {}
