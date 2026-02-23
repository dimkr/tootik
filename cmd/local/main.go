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
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/dimkr/tootik/cluster"
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

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s USERNAME PATH INPUT\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Simulates a local social network.")
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
		os.Exit(2)
	}
	flag.Parse()

	if flag.NArg() != 3 {
		flag.Usage()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

	username := os.Args[1]

	cert, err := generateCert(username)
	if err != nil {
		slog.Error("Failed to create temporary directory")
		os.Exit(1)
	}

	path := os.Args[2]
	input := os.Args[3]

	os.Stdout.WriteString(cl["local.example"].HandleInput(cert, path, input).Raw)
}
