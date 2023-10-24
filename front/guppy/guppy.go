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
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/guppy"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/url"
	"sync"
	"time"
)

type incomingPacket struct {
	Data []byte
	From net.Addr
}

type responseChunk struct {
	Data  []byte
	Seq   int
	Acked bool
	Sent  time.Time
}

const (
	maxRequestSize    = 1024
	responseChunkSize = 512
	maxUnackedChunks  = 8
	reqTimeout        = time.Second * 30
	maxSessions       = 32
	resendInterval    = time.Second * 2
)

func handle(ctx context.Context, log *slog.Logger, db *sql.DB, handler front.Handler, resolver *fed.Resolver, wg *sync.WaitGroup, from net.Addr, req []byte, acks <-chan []byte, done chan<- string, s net.PacketConn) {
	defer func() {
		done <- from.String()
	}()

	if req[len(req)-1] == '\r' && req[len(req)] == '\n' {
		log.Warn("Invalid request")
		return
	}

	reqUrl, err := url.Parse(string(req[:len(req)-2]))
	if err != nil {
		log.Warn("Invalid request", "request", string(req[:len(req)-2]), "error", err)
		return
	}

	seq := 2 + rand.Intn(math.MaxInt16/2)

	var buf bytes.Buffer
	w := guppy.Wrap(&buf, seq)

	if reqUrl.Host != cfg.Domain {
		w.Status(1, "Wrong host")
	} else {
		log.Info("Handling request", "path", reqUrl.Path, "from", from)
		handler.Handle(ctx, log, w, reqUrl, nil, db, resolver, wg)
	}

	if ctx.Err() != nil {
		log.Warn("Failed to handle request in time", "path", reqUrl.Path, "from", from)
		return
	}

	chunk := make([]byte, responseChunkSize)

	n, err := buf.Read(chunk)
	if err != nil {
		log.Error("Failed to read first respone chunk", "error", err)
		return
	}

	chunks := make([]responseChunk, 1, buf.Len()/responseChunkSize+2)
	chunks[0].Seq = seq
	chunks[0].Data = chunk[:n]

	// fix the sequence number if the response is cached
	// TODO: something less ugly
	space := bytes.IndexByte(chunk[:n], ' ')
	if string(chunk[:space]) != "0" && string(chunk[:space]) != "1" {
		chunks[0].Data = append([]byte(fmt.Sprintf("%d", seq)), chunk[space:n]...)
	}

	for {
		seq++
		statusLine := fmt.Sprintf("%d\r\n", seq)
		n, err := buf.Read(chunk)
		if err != nil && errors.Is(err, io.EOF) {
			// this is the EOF packet
			chunks = append(chunks, responseChunk{Data: []byte(statusLine), Seq: seq})
			break
		} else if err != nil {
			log.Error("Failed to read respone chunk", "error", err)
			return
		}
		chunks = append(chunks, responseChunk{Data: append([]byte(statusLine), chunk[:n]...), Seq: seq})
	}

	retry := time.NewTicker(resendInterval)
	defer retry.Stop()

	log.Debug("Sending response", "path", reqUrl.Path, "from", from, "first", chunks[0].Seq, "last", chunks[len(chunks)-1].Seq, "chunks", len(chunks))

	send := true

	for {
		select {
		case <-ctx.Done():
			log.Warn("Session timed out", "path", reqUrl.Path, "from", from)
			return

		case ack, ok := <-acks:
			if !ok {
				log.Warn("Session timed out", "path", reqUrl.Path, "from", from)
				return
			}

			var ackedSeq int
			n, err := fmt.Sscanf(string(ack), "%d\r\n", &ackedSeq)
			if err != nil {
				log.Debug("Received invalid ack", "path", reqUrl.Path, "from", from, "ack", string(ack), "error", err)
				continue
			}
			if n < 1 {
				log.Debug("Received invalid ack", "path", reqUrl.Path, "from", from, "ack", string(ack))
				continue
			}

			i := ackedSeq - chunks[0].Seq
			if i < 0 || i >= len(chunks) {
				log.Debug("Received invalid ack", "path", reqUrl.Path, "from", from, "ack", string(ack))
				continue
			}

			if chunks[i].Acked {
				log.Debug("Received duplicate ack", "path", reqUrl.Path, "from", from, "acked", ackedSeq)
				continue
			}

			log.Debug("Marking packet as received", "path", reqUrl.Path, "from", from, "acked", ackedSeq)
			chunks[i].Acked = true

			// stop if the acked packet is the EOF packet
			if i == len(chunks)-1 {
				return
			}

			send = true
			retry.Reset(resendInterval)

		case <-retry.C:
			send = true
		}

		if !send {
			continue
		}

		now := time.Now()
		sent := 0
		for _, chunk := range chunks {
			if chunk.Acked || now.Sub(chunk.Sent) <= resendInterval {
				continue
			}
			if sent == maxUnackedChunks {
				break
			}
			if chunk.Sent == (time.Time{}) {
				log.Debug("Sending packet", "path", reqUrl.Path, "from", from, "seq", chunk.Seq)
			} else {
				log.Debug("Resending packet", "path", reqUrl.Path, "from", from, "seq", chunk.Seq)
			}
			s.WriteTo(chunk.Data, from)
			chunk.Sent = now
			sent++
		}

		send = false
	}
}

func ListenAndServe(ctx context.Context, log *slog.Logger, db *sql.DB, handler front.Handler, resolver *fed.Resolver, addr string) error {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	incoming := make(chan incomingPacket)

	wg.Add(1)
	go func() {
		defer close(incoming)
		defer wg.Done()

		buf := make([]byte, maxRequestSize)
		for {
			n, from, err := l.ReadFrom(buf)
			if err != nil {
				log.Error("Failed to receive a packet", "error", err)
				return
			}
			incoming <- incomingPacket{buf[:n], from}
		}
	}()

	sessions := make(map[string]chan []byte)
	done := make(chan string, 1)

loop:
	for {
		keepClosing := true

		for keepClosing {
			select {
			case k := <-done:
				acks := sessions[k]
				close(acks)
				delete(sessions, k)

			default:
				keepClosing = false
			}
		}

		select {
		case <-ctx.Done():
			break loop

		case pkt, ok := <-incoming:
			if !ok {
				break loop
			}
			k := pkt.From.String()

			if acks, ok := sessions[k]; ok {
				if len(acks) < maxUnackedChunks {
					acks <- pkt.Data
				}
				continue
			}
			if len(sessions) > maxSessions {
				log.Warn("Too many sessions")
				l.WriteTo([]byte("1 Too many sessions\r\n"), pkt.From)
				continue
			}

			acks := make(chan []byte, maxUnackedChunks)
			sessions[k] = acks

			requestCtx, cancelRequest := context.WithTimeout(ctx, reqTimeout)

			wg.Add(1)
			go func() {
				handle(requestCtx, log, db, handler, resolver, &wg, pkt.From, pkt.Data, acks, done, l)
				cancelRequest()
				wg.Done()
			}()
		}
	}

	l.Close()
	wg.Wait()
	return nil
}
