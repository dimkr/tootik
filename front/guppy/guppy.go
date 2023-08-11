/*
Copyright 2023 Dima Krasner

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

package guppy

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front"
	log "github.com/dimkr/tootik/slogru"
	"github.com/dimkr/tootik/text/guppy"
	"io"
	"math"
	"math/rand"
	"net"
	"net/url"
	"sync"
	"time"
)

const (
	maxRequestSize    = 1024
	responseChunkSize = 512
	reqTimeout        = time.Second * 30
	maxSessions       = 32
	retryInterval     = time.Second * 2
)

type packet struct {
	Payload []byte
	From    net.Addr
}

func handle(ctx context.Context, db *sql.DB, wg *sync.WaitGroup, from net.Addr, req []byte, acks <-chan []byte, done chan<- string, s net.PacketConn) {
	defer func() {
		done <- from.String()
	}()

	if req[len(req)-1] == '\r' && req[len(req)] == '\n' {
		log.Warn("Invalid request")
		return
	}

	reqUrl, err := url.Parse(string(req[:len(req)-2]))
	if err != nil {
		log.WithField("request", string(req[:len(req)-2])).WithError(err).Warn("Invalid request")
		return
	}

	seq := 2 + rand.Intn(math.MaxInt16/2)

	var buf bytes.Buffer
	w := guppy.Wrap(&buf, seq)

	if reqUrl.Host != cfg.Domain {
		w.Status(1, "Wrong host")
	} else {
		log.WithFields(log.Fields{"path": reqUrl.Path, "from": from}).Info("Handling request")
		front.Handle(ctx, w, reqUrl, nil, db, wg)
	}

	if ctx.Err() != nil {
		log.WithFields(log.Fields{"path": reqUrl.Path, "from": from}).Warn("Failed to handle request in time")
		return
	}

	chunk := make([]byte, responseChunkSize)

	n, err := buf.Read(chunk)
	if err != nil {
		log.WithError(err).Error("Failed to read first respone chunk")
		return
	}

	prevPacket := chunk[:n]

	// fix the sequence number if the response is cached
	// TODO: something less ugly
	space := bytes.IndexByte(chunk[:n], ' ')
	if string(chunk[:space]) != "0" && string(chunk[:space]) != "1" {
		prevPacket = append([]byte(fmt.Sprintf("%d", seq)), chunk[space:n]...)
	}

	s.WriteTo(prevPacket, from)

	retry := time.NewTicker(retryInterval)
	defer retry.Stop()

	expectedAck := fmt.Sprintf("%d\r\n", seq)
	finSent := false

	for {
		select {
		case <-ctx.Done():
			log.WithFields(log.Fields{"path": reqUrl.Path, "from": from}).Warn("Session timed out")
			return

		case ack, ok := <-acks:
			if !ok {
				log.WithFields(log.Fields{"path": reqUrl.Path, "from": from}).Warn("Session timed out")
				return
			}

			if string(ack) != expectedAck {
				log.WithFields(log.Fields{"path": reqUrl.Path, "from": from, "expected": expectedAck, "got": string(ack)}).Debug("Received invalid ack")
				continue
			}

			if finSent {
				return
			}

			seq++
			statusLine := fmt.Sprintf("%d\r\n", seq)
			n, err := buf.Read(chunk)
			if err != nil && errors.Is(err, io.EOF) {
				prevPacket = []byte(statusLine)
			} else if err != nil {
				log.WithError(err).Error("Failed to read respone chunk")
				return
			} else {
				prevPacket = append([]byte(statusLine), chunk[:n]...)
			}

			s.WriteTo(prevPacket, from)

			finSent = n == 0
			expectedAck = statusLine

			retry.Reset(retryInterval)

		case <-retry.C:
			log.WithFields(log.Fields{"path": reqUrl.Path, "from": from}).Debug("Resending previous packet")
			s.WriteTo(prevPacket, from)
		}
	}
}

func ListenAndServe(ctx context.Context, db *sql.DB, addr string) error {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	packets := make(chan packet)

	wg.Add(1)
	go func() {
		buf := make([]byte, maxRequestSize)
		for {
			n, from, err := l.ReadFrom(buf)
			if err != nil {
				log.WithError(err).Warn("Failed to receive a packet")
				continue
			}
			packets <- packet{buf[:n], from}
		}
	}()

	sessions := make(map[string]chan []byte)
	done := make(chan string, 1)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case k := <-done:
			acks := sessions[k]
			close(acks)
			delete(sessions, k)

		case pkt := <-packets:
			k := pkt.From.String()

			if acks, ok := sessions[k]; ok {
				acks <- pkt.Payload
				continue
			}
			if len(sessions) > maxSessions {
				log.Warn("Too many sessions")
				l.WriteTo([]byte("1 Too many sessions\r\n"), pkt.From)
				continue
			}

			acks := make(chan []byte, 1)
			sessions[k] = acks

			requestCtx, cancelRequest := context.WithTimeout(ctx, reqTimeout)

			wg.Add(1)
			go func() {
				handle(requestCtx, db, &wg, pkt.From, pkt.Payload, acks, done, l)
				cancelRequest()
				wg.Done()
			}()
		}
	}

	l.Close()
	wg.Wait()
	return nil
}
