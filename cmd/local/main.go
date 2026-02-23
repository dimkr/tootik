/*
Copyright 2026 Dima Krasner

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

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dimkr/tootik/cluster"
	"github.com/dimkr/tootik/front/text"
	"golang.org/x/term"
)

func generateCert(user string) (tls.Certificate, error) {
	hash := sha256.Sum256([]byte(user))

	privateKey := ed25519.NewKeyFromSeed(hash[:])

	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName: user,
		},
		SerialNumber: new(big.Int),
		NotBefore:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		privateKey.Public(),
		privateKey,
	)
	if err != nil {
		return tls.Certificate{}, err
	}

	var certPEM bytes.Buffer
	if err := pem.Encode(&certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return tls.Certificate{}, err
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	var keyPEM bytes.Buffer
	if err := pem.Encode(&keyPEM, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(certPEM.Bytes(), keyPEM.Bytes())
}

func render(p cluster.Page) ([]string, []string) {
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols = 80
	}

	var lines, links []string
	linkID := 1
	for _, l := range p.Lines {
		switch l.Type {
		case cluster.Heading:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				lines = append(lines, "\033[4m# "+line+"\033[0m")
			}

		case cluster.SubHeading:
			for _, line := range text.WordWrap(l.Text, cols-3, -1) {
				lines = append(lines, "\033[4m## "+line+"\033[0m")
			}

		case cluster.Quote:
			for _, line := range text.WordWrap(l.Text, cols-2, -1) {
				lines = append(lines, "> "+line)
			}

		case cluster.Item:
			for i, line := range text.WordWrap(l.Text, cols-2, -1) {
				if i == 0 {
					lines = append(lines, "* "+line)
				} else {
					lines = append(lines, " "+line)
				}
			}

		case cluster.Link:
			prefix := fmt.Sprintf("[%d] ", linkID)
			for i, line := range text.WordWrap(l.Text, cols-len(prefix), -1) {
				if i == 0 {
					lines = append(lines, fmt.Sprintf("\033[4;36m[%d]\033[0;39m %s", linkID, line))
				} else {
					lines = append(lines, strings.Repeat(" ", len(prefix))+line)
				}
			}
			links = append(links, l.URL)
			linkID++

		case cluster.Preformatted:
			lines = append(lines, text.WordWrap(l.Text, cols, -1)[0])

		default:
			lines = append(lines, text.WordWrap(l.Text, cols, -1)...)
		}
	}

	return lines, links
}

func shell(ctx context.Context, server *cluster.Server, user string) error {
	cert, err := generateCert(user)
	if err != nil {
		return err
	}

	var links []string

	p := server.Handle(cert, "/users")

	bestlineSetHintsCallback(func(text string, ansi1, ansi2 *string) string {
		if text == "" && len(links) > 0 {
			*ansi1 = "\033[90m"
			*ansi2 = "\033[0m"
			return fmt.Sprintf(" 1-%d", len(links))
		} else if len(links) == 0 {
			return ""
		}

		if n, err := strconv.Atoi(text); err == nil && n > 0 {
			i := 0
			for _, line := range p.Lines {
				if line.Type != cluster.Link {
					continue
				}

				i++
				if i == n {
					*ansi1 = "\033[90m"
					*ansi2 = "\033[0m"
					return " " + line.Text
				}
			}
		}

		return ""
	})

	for {
		if err := ctx.Err(); err != nil {
			break
		}

		var lines, history []string
		lines, links = render(p)

		if len(lines) > 0 {
			c := exec.CommandContext(ctx, "less", "-r")
			c.Stdin = strings.NewReader(strings.Join(lines, "\n"))
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
		}

		prompt := ">"
		if strings.HasPrefix(p.Status, "10 ") {
			prompt = p.Status[3:]
		} else {
			for _, line := range p.Lines {
				if line.Type == cluster.Heading {
					prompt = line.Text
					break
				}
			}
		}

		line, err := bestline("\033[35m%s>\033[0m ", prompt)
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if n, err := strconv.Atoi(line); err == nil && n > 0 && n <= len(links) {
			nextURL := links[n-1]
			u, err := url.Parse(nextURL)
			if err != nil {
				return err
			}
			history = append(history, p.Path)
			p = p.Goto(u.String())
		} else if strings.HasPrefix(p.Status, "10 ") {
			u, err := url.Parse(p.Path)
			if err != nil {
				return err
			}
			u.RawQuery = url.QueryEscape(line)
			history = append(history, p.Path)
			p = p.Goto(u.String())
		} else {
			u, err := url.Parse(line)
			if err != nil {
				fmt.Printf("Invalid URL or command: %s\n", line)
				continue
			}
			history = append(history, p.Path)
			p = p.Goto(u.String())
		}
	}

	return nil
}

func main() {
	shellMode := flag.Bool("shell", false, "Start an interactive shell")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-shell | USERNAME PATH INPUT]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Simulates a local social network.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "  -shell")
		fmt.Fprintln(flag.CommandLine.Output(), "    Start an interactive shell as the current OS user.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "USERNAME is the user to authenticate as.")
		fmt.Fprintln(flag.CommandLine.Output(), "PATH is a Gemini path (e.g. /users, /local, /users/say).")
		fmt.Fprintln(flag.CommandLine.Output(), "INPUT is user input for the request (use \"\" for none).")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "The response is a Gemini protocol response containing a gemtext")
		fmt.Fprintln(flag.CommandLine.Output(), "document, printed to stdout.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Users can publish public posts, posts to followers or direct messages,")
		fmt.Fprintln(flag.CommandLine.Output(), "reply to, edit, delete and share posts, follow and unfollow users,")
		fmt.Fprintln(flag.CommandLine.Output(), "publish polls, browse local posts and hashtags, perform full-text")
		fmt.Fprintln(flag.CommandLine.Output(), "search and edit their profile.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Non-existing users must register first:")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s USERNAME /users/register generate\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "New users should read /users/help for more information.")
		os.Exit(2)
	}
	flag.Parse()

	if !*shellMode && flag.NArg() != 3 {
		flag.Usage()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.SetDefault(slog.New(slog.DiscardHandler))

	var baseDir string
	if runtime.GOOS == "linux" {
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				slog.Error("Failed to determine home directory")
				os.Exit(1)
			}
			baseDir = filepath.Join(home, ".local", "share")
		}
	} else {
		var err error
		baseDir, err = os.UserConfigDir()
		if err != nil {
			slog.Error("Failed to determine data directory")
			os.Exit(1)
		}
	}

	dataDir := filepath.Join(baseDir, "tootik")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		slog.Error("Failed to create data directory")
		os.Exit(1)
	}

	t := t{tempDir: dataDir, ctx: ctx}

	cl := cluster.NewCluster(t, "local.example")

	cl["local.example"].Config.PostThrottleUnit = 1
	cl["local.example"].Config.EditThrottleUnit = 1
	cl["local.example"].Config.ShareThrottleUnit = 1
	cl["local.example"].Config.MinBookmarkInterval = 1
	cl["local.example"].Config.MinActorEditInterval = 1

	cl["local.example"].Config.MaxPostsLength = 1024 * 1024 * 1024
	cl["local.example"].Config.MaxPostsPerDay = 1024

	if *shellMode {
		u, err := user.Current()
		if err != nil {
			slog.Error("Failed to determine current user")
			os.Exit(1)
		}

		if err := shell(ctx, cl["local.example"], u.Username); err != nil {
			slog.Error("Shell error", "error", err)
			os.Exit(1)
		}
		return
	}

	username := flag.Arg(0)

	cert, err := generateCert(username)
	if err != nil {
		slog.Error("Failed to create temporary directory")
		os.Exit(1)
	}

	path := flag.Arg(1)
	input := flag.Arg(2)

	os.Stdout.WriteString(cl["local.example"].HandleInput(cert, path, input).Raw)
}
