package transfer

import (
	"errors"
	"io"
	"testing"
	"time"
)

func readAll(t *testing.T, r io.Reader) error {
	t.Helper()
	done := make(chan error, 1)

	go func() {
		_, err := io.ReadAll(r)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("the reader was left waiting")
		return nil
	}
}

func TestFinishClosesTheWriterWithTheError(t *testing.T) {
	r, w := io.Pipe()
	j := &Job{Writer: w}
	lost := errors.New("session dropped")

	j.finish(Failed, lost)

	if err := readAll(t, r); !errors.Is(err, lost) {
		t.Fatalf("reader got %v, want the failure so a half file isn't kept", err)
	}

	if !errors.Is(j.Err(), lost) {
		t.Fatalf("Err = %v", j.Err())
	}

	j.finish(Failed, lost)
}

func TestFinishClosesTheWriterCleanlyOnSuccess(t *testing.T) {
	r, w := io.Pipe()
	j := &Job{Writer: w}

	j.finish(Done, nil)

	if err := readAll(t, r); err != nil {
		t.Fatalf("reader got %v, want a clean end", err)
	}
}
