package download

import (
	"io"
	"os"
)

type offsetWriter struct {
	file   *os.File
	offset int64
}

func (w *offsetWriter) Write(p []byte) (int, error) {
	n, err := w.file.WriteAt(p, w.offset)
	w.offset += int64(n)
	return n, err
}

type passThroughWriter struct {
	Writer  io.Writer
	OnWrite func(n int)
}

func (w *passThroughWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if n > 0 && w.OnWrite != nil {
		w.OnWrite(n)
	}
	return n, err
}
