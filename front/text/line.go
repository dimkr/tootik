/*
Copyright 2024 Dima Krasner

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

const bufferSize = 16

func LineBuffered(inner io.Writer) *LineWriter {
	w := &LineWriter{
		inner: inner,
		c:     make(chan []byte),
		done:  make(chan error, 1),
	}

	go func() {
		lines := 0

		for {
			buf, ok := <-w.c
			if !ok {
				break
			}

			if len(buf) == 0 {
				continue
			}

			w.buffer.Write(buf)

			if buf[len(buf)-1] != '\n' {
				continue
			}

			lines++

			if lines == bufferSize {
				_, err := w.inner.Write(w.buffer.Bytes())
				if err != nil {
					w.done <- err
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
	if !w.closed {
		close(w.c)
		err := <-w.done
		w.closed = true
		w.err = err
	}

	if w.err != nil {
		return w.err
	}

	return net.ErrClosed
}
