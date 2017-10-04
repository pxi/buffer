package buffer

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pxi/is"
)

const (
	w1 = "aa"
	w2 = "bb"
	w3 = "cc"
)

func TestBuffer(t *testing.T) {
	b := &Buffer{}

	testRead := func(r io.Reader, want string, wantErr error) {
		t.Helper()
		s, err := read(r, len(w1+w2+w3)*2)
		is.Equal(t, s, want)
		is.Equal(t, err, wantErr)
	}

	var wg sync.WaitGroup
	testConc := func(r io.Reader, want string, wantErr error) {
		wg.Add(1)
		go func() {
			// TODO(pxi) figure out how to use t.Helper() with
			// the goroutine here.
			defer wg.Done()
			testRead(r, want, wantErr)
		}()
	}

	r1 := NewReader(b)
	testConc(r1, w1, nil)

	// Write first word.
	time.Sleep(time.Millisecond)
	is.Ok(t, write(b, w1))

	wg.Wait()

	// Write second word.
	is.Ok(t, write(b, w2))
	testRead(r1, w2, nil)

	// Second reader reads the whole buffer.
	r2 := NewReader(b)
	testRead(r2, w1+w2, nil)

	wg.Wait()

	// Both readers waiting for third word.
	testConc(r1, w3, nil)
	testConc(r2, w3, nil)
	time.Sleep(time.Millisecond)
	is.Ok(t, write(b, w3))

	wg.Wait()

	// Third reader reads the whole buffer.
	r3 := NewReader(b)
	testRead(r3, w1+w2+w3, nil)

	// First reader waits for EOF.
	testConc(r1, "", io.EOF)

	// Close the buffer and trigger EOF to the first reader.
	time.Sleep(time.Millisecond)
	is.Ok(t, b.Close())

	// Check that write fails after closing the buffer.
	is.Equal(t, write(b, "something"), errClosed)

	// Second reader gets EOF.
	testRead(r2, "", io.EOF)

	wg.Wait()

	// Fourth reader waits unexpected EOF.
	r4 := NewReader(b)

	// Reset the buffer.
	b.Reset()
	is.Equal(t, b.Len(), 0)
	is.Content(t, b.Cap() > 0, "buffer is not retained")

	// Third reader receives unexpected EOF.
	testRead(r3, "", io.ErrUnexpectedEOF)
	testRead(r4, "", io.ErrUnexpectedEOF)
}

func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

func read(r io.Reader, c int) (string, error) {
	p := make([]byte, c)
	n, err := r.Read(p)
	return string(p[:n]), err
}
