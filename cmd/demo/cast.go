package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
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
	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}

	if _, err := fmt.Fprintf(w, "{\"version\": 2, \"width\": %d, \"height\": %d, \"env\": {\"SHELL\": \"/bin/bash\", \"TERM\": \"%s\"}}\n", cols, rows, term); err != nil {
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

func (c *cast) Input(s string) error {
	_, err := c.rawPty.Write([]byte(s))
	return err
}

func (c *cast) output(b []byte) error {
	return c.record('o', b)
}

func (c *cast) watch() error {
	buf := make([]byte, 1024*1024)

	for {
		n, err := c.rawPty.Read(buf)
		if err != nil {
			//return err
			return nil
		}

		if err := c.output(buf[:n]); err != nil {
			return err
		}
	}
}

func (c *cast) Down(ctx context.Context, times int) error {
	for range times {
		if err := c.Input("\x1bOB"); err != nil {
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
	return c.Input("\x1b[6~")
}

func (c *cast) Enter() error {
	return c.Input("\r")
}

func (c *cast) Type(ctx context.Context, s string) error {
	for i, r := range s {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond * time.Duration(30+rand.N(max(200-(60*(i/4)), 30)))):
			}
		}

		if err := c.Input(string([]rune{r})); err != nil {
			return err
		}
	}

	return nil
}
