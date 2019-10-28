package writecounter

import (
	"io"
	"sync/atomic"
)

// Lib

type WriteCounter struct {
	io.Writer
	pos    uint64
	writer io.Writer
}

func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{writer: w}
}

func (counter *WriteCounter) Write(buf []byte) (int, error) {
	n, err := counter.writer.Write(buf)
	atomic.AddUint64(&counter.pos, uint64(n))
	return n, err
}

func (counter *WriteCounter) Pos() uint64 {
	return atomic.LoadUint64(&counter.pos)
}

// End Lib
