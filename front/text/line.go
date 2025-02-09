/*
Copyright 2024, 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package text

import (
	"bytes"
	"io"
	"net"
	"slices"
	"time"
)

// LineWriter wraps an [io.Writer] with line-based buffering and writes in a separate routine.
type LineWriter struct {
	inner  io.Writer
	buffer bytes.Buffer
	c      chan []byte
	closed bool
	done   chan error
	err    error
}

const (
	bufferSize    = 16
	flushInterval = time.Millisecond * 100
)

func LineBuffered(inner io.Writer) *LineWriter {
	w := &LineWriter{
		inner: inner,
		c:     make(chan []byte),
		done:  make(chan error, 1),
	}

	go func() {
		lines := 0

		t := time.NewTicker(flushInterval)
		defer t.Stop()

		drain := func() {
			for {
				select {
				case <-t.C:
				default:
					return
				}
			}
		}

	loop:
		for {
			select {
			case buf, ok := <-w.c:
				if !ok {
					break loop
				}

				if len(buf) == 0 {
					continue
				}

				for {
					if len(buf) == 0 {
						break
					}

					i := bytes.IndexByte(buf, '\n')
					if i == -1 {
						w.buffer.Write(buf)
						t.Stop()
						drain()
						break
					}

					w.buffer.Write(buf[:i+1])
					lines++

					// flush if we have $bufferSize lines in the buffer
					if lines == bufferSize {
						t.Stop()
						drain()

						if _, err := w.inner.Write(w.buffer.Bytes()); err != nil {
							w.stop(err)
							return
						}

						w.buffer.Reset()
						lines = 0
					} else if lines > 0 {
						t.Reset(flushInterval)
					}

					buf = buf[i+1:]
				}

				// flush if we have lines waiting for >=$flushInterval in the buffer
			case <-t.C:
				buf := w.buffer.Bytes()

				if len(buf) == 0 || buf[len(buf)-1] != '\n' {
					continue
				}

				t.Stop()
				drain()

				if _, err := w.inner.Write(buf); err != nil {
					w.stop(err)
					return
				}

				w.buffer.Reset()
				lines = 0
			}
		}

		if w.buffer.Len() > 0 {
			if _, err := w.inner.Write(w.buffer.Bytes()); err != nil {
				w.done <- err
				return
			}
		}

		w.done <- nil
	}()

	return w
}

func (w *LineWriter) stop(err error) {
	// continue reading until closed, to unblock the writing routine
	for {
		if _, ok := <-w.c; !ok {
			break
		}
	}

	w.done <- err
}

func (w *LineWriter) Unwrap() io.Writer {
	return w.inner
}

func (w *LineWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}

	if w.closed {
		return 0, net.ErrClosed
	}

	w.c <- slices.Clone(p)
	return len(p), nil
}

func (w *LineWriter) Flush() error {
	wasClosed := w.closed

	if !w.closed {
		close(w.c)
		err := <-w.done
		w.closed = true
		w.err = err
	}

	if w.err != nil {
		return w.err
	}

	if !wasClosed {
		return nil
	}

	return net.ErrClosed
}
