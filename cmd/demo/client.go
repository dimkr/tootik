package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
)

const maxRedirects = 5

var geminiStatusLineRegex = regexp.MustCompile(`^(\d\d)\s+(.*?)\s*$`)

func newDialer(cert tls.Certificate) *tls.Dialer {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	return &tls.Dialer{Config: cfg}
}

type geminiClient struct {
	here   *url.URL
	Prompt string
}

func sendGeminiRequest(
	ctx context.Context,
	conn net.Conn,
	u *url.URL,
) (int64, string) {
	stop := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	defer func() {
		stop <- struct{}{}
		<-stopped
	}()

	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-stop:
		}

		stopped <- struct{}{}
	}()

	if _, err := fmt.Fprintf(conn, "%s\r\n", u.String()); err != nil {
		panic(err)
	}

	buf := make([]byte, 0, 128)
	for {
		if _, err := conn.Read(buf[len(buf) : len(buf)+1]); err != nil {
			return 0, ""
		}

		buf = buf[:len(buf)+1]

		if len(buf) >= 2 && buf[len(buf)-2] == '\r' && buf[len(buf)-1] == '\n' {
			break
		}

		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
	}

	buf = buf[:len(buf)-2]

	m := geminiStatusLineRegex.FindStringSubmatch(string(buf))
	if m == nil {
		panic(string(buf))
	}

	statusCode, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		panic(err)
	}

	return statusCode, m[2]
}

func connectAndSendRequest(
	ctx context.Context,
	u *url.URL,
	hostPort string,
	dialer *tls.Dialer,
) (net.Conn, int64, string) {
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		panic(err)
	}

	statusCode, meta := sendGeminiRequest(ctx, conn, u)
	return conn, statusCode, meta
}

func (c *geminiClient) Download(ctx context.Context, u *url.URL, redirs uint, cert tls.Certificate) net.Conn {
	if c.here != nil {
		u = c.here.ResolveReference(u)
	}

	host := u.Hostname()

	port := u.Port()
	if port == "" {
		port = "1965"
	}

	hostPort := host + ":" + port

	dialer := newDialer(cert)

	for {
		conn, statusCode, meta := connectAndSendRequest(ctx, u, hostPort, dialer)

		if statusCode >= 20 && statusCode < 30 {
			c.here = u
			c.Prompt = ""
			return conn
		} else if statusCode >= 30 && statusCode < 40 {
			conn.Close()

			if redirs == maxRedirects {
				panic("too many redirects")
			}
			redirs++

			rel, err := url.Parse(meta)
			if err != nil {
				panic(err)
			}

			u = u.ResolveReference(rel)
		} else if statusCode >= 10 && statusCode < 20 {
			c.Prompt = meta
			return conn
		} else {
			panic(statusCode)
		}
	}
}

func (c *geminiClient) downloadPage(ctx context.Context, urlString string, cert tls.Certificate) string {
	u, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}

	conn := c.Download(ctx, u, 0, cert)

	buf, err := io.ReadAll(conn)
	if err != nil {
		panic(err)
	}

	return string(buf)
}
