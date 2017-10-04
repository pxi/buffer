// Package buffer provides a buffer that can be written to by one writer and
// read from by many readers.
package buffer

import (
	"errors"
	"io"
	"sync"
)

// Buffer is a variable-sized buffer of bytes.
type Buffer struct {
	mu  sync.RWMutex
	buf []byte
	eof bool
	set bool
	sig *sync.Cond
}

// Len returns the number of bytes written to buffer.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.buf)
}

// Cap returns the capacity allocated for the buffer.
func (b *Buffer) Cap() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return cap(b.buf)
}

// Bytes returns a copy of the underlying buffer.
func (b *Buffer) Bytes() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]byte(nil), b.buf...)
}

// String returns a copy of the underlying buffer as a string.
func (b *Buffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return string(b.buf)
}

// errClosed is returned from Write if the buffer is closed.
var errClosed = errors.New("buffer: write on closed buffer")

// Write appends the contents of p to the buffer, growing it as needed.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.eof {
		return 0, errClosed
	}

	if b.buf == nil {
		// TODO(pxi) make initial cap configurable
		b.buf = make([]byte, 0, 1024)
	}

	b.buf = append(b.buf, p...)
	b.signal()

	return len(p), nil
}

// Close closes buffer from writing and signals EOF to all readers.
func (b *Buffer) Close() error {
	b.mu.Lock()
	if !b.eof {
		b.eof = true
		b.signal()
	}
	b.mu.Unlock()
	return nil
}

// Reset resets the buffer retaining allocated space. Current readers return
// unexpected EOF as the data stream is discontinued.
func (b *Buffer) Reset() {
	b.mu.Lock()
	b.eof = false
	b.buf = b.buf[:0]
	b.set = !b.set
	b.signal()
	b.mu.Unlock()
}

func (b *Buffer) signal() {
	if b.sig != nil {
		b.sig.Broadcast()
	}
}

type reader struct {
	*Buffer
	off  int
	mark bool
}

// NewReader returns a new io.Reader that will emit the whole b.
func NewReader(b *Buffer) io.Reader {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sig == nil {
		b.sig = sync.NewCond(b.mu.RLocker())
	}
	return &reader{Buffer: b, mark: b.set}
}

func (r *reader) Read(p []byte) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Wait for more data or EOF or reset.
	for (!r.eof && len(r.buf) == r.off) && (r.mark == r.set) {
		r.sig.Wait()
	}

	// Return unexpected eof if buffer was reset.
	if r.mark != r.set {
		return 0, io.ErrUnexpectedEOF
	}

	// Return EOF if buffer reported EOF.
	if len(r.buf) == r.off && r.eof {
		return 0, io.EOF
	}

	n := copy(p, r.buf[r.off:])
	r.off += n

	return n, nil
}
