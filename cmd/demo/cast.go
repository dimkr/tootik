package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"sync"
	"time"
)

type cast struct {
	start  time.Time
	lock   sync.Mutex
	w      io.Writer
	rawPty io.ReadWriter
	done   <-chan error
}

func startCast(rawPty io.ReadWriter, w io.Writer, cols, rows int) (*cast, error) {
	if _, err := fmt.Fprintf(w, "{\"version\": 2, \"width\": %d, \"height\": %d, \"env\": {\"SHELL\": \"/bin/bash\", \"TERM\": \"xterm-256color\"}}\n", cols, rows); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	c := &cast{rawPty: rawPty, w: w, done: done}
	go func() {
		done <- c.watch()
	}()

	return c, nil
}

func (c *cast) Wait() error {
	return <-c.done
}

func (c *cast) record(code byte, buf []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if code == 'i' {
		if _, err := c.rawPty.Write(buf); err != nil {
			return err
		}
	}

	var delta time.Duration
	if c.start.IsZero() {
		c.start = time.Now()
	} else {
		delta = time.Since(c.start)
	}

	j, err := json.Marshal(string(buf))
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(c.w, "[%.6f, \"%c\", %s]\n", float64(delta)/float64(time.Second), code, string(j)); err != nil {
		return err
	}

	return nil
}

func (c *cast) watch() error {
	buf := make([]byte, 1024*1024)

	for {
		n, err := c.rawPty.Read(buf)
		if err != nil {
			//return err
			return nil
		}

		if err := c.record('o', buf[:n]); err != nil {
			return err
		}
	}
}

func (c *cast) Down(ctx context.Context, times int) error {
	for range times {
		if err := c.record('i', []byte("\x1b[B")); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * time.Duration(50+rand.UintN(150))):
		}
	}

	return nil
}

func (c *cast) PageDown() error {
	return c.record('i', []byte("\x1b[6~"))
}

func (c *cast) Enter() error {
	return c.record('i', []byte("\r"))
}

func (c *cast) Type(ctx context.Context, s string) error {
	for i, r := range s {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond * time.Duration(100+rand.UintN(200))):
			}
		}

		if err := c.record('i', []byte(string([]rune{r}))); err != nil {
			return err
		}
	}

	return nil
}
