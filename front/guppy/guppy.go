/*
Copyright 2023, 2024 Dima Krasner

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
	maxRequestSize = 1024
	retryInterval  = time.Millisecond * 100
)

func handle(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, db *sql.DB, handler front.Handler, resolver *fed.Resolver, wg *sync.WaitGroup, from net.Addr, req []byte, acks <-chan []byte, done chan<- string, s net.PacketConn) {
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

	seq := 6 + rand.Intn(math.MaxInt16/2)

	var buf bytes.Buffer
	w := guppy.Wrap(&buf, seq)

	if reqUrl.Host != domain {
		w.Status(4, "Wrong host")
	} else {
		log.Info("Handling request", "path", reqUrl.Path, "url", reqUrl.String(), "from", from)
		handler.Handle(ctx, log, w, reqUrl, nil, db, resolver, wg)
	}

	if ctx.Err() != nil {
		log.Warn("Failed to handle request in time", "path", reqUrl.Path, "from", from)
		return
	}

	chunk := make([]byte, cfg.GuppyResponseChunkSize)

	n, err := buf.Read(chunk)
	if err != nil {
		log.Error("Failed to read first respone chunk", "error", err)
		return
	}

	chunks := make([]responseChunk, 1, buf.Len()/cfg.GuppyResponseChunkSize+2)
	chunks[0].Seq = seq
	chunks[0].Data = chunk[:n]

	// fix the sequence number if the response is cached
	// TODO: something less ugly
	space := bytes.IndexByte(chunk[:n], ' ')
	if string(chunk[:space]) != "1" && string(chunk[:space]) != "3" && string(chunk[:space]) != "4" {
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

	retry := time.NewTicker(retryInterval)
	defer retry.Stop()

	log.Debug("Sending response", "path", reqUrl.Path, "from", from, "first", chunks[0].Seq, "last", chunks[len(chunks)-1].Seq, "chunks", len(chunks))

	firstTime := true

	for {
		if !firstTime {
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

			case <-retry.C:
			}
		}

		now := time.Now()
		sent := 0
		for i := range chunks {
			if chunks[i].Acked || now.Sub(chunks[i].Sent) <= cfg.GuppyChunkTimeout {
				continue
			}
			if sent == cfg.MaxSentGuppyChunks {
				break
			}
			if chunks[i].Sent == (time.Time{}) {
				log.Debug("Sending packet", "path", reqUrl.Path, "from", from, "seq", chunks[i].Seq)
			} else {
				log.Debug("Resending packet", "path", reqUrl.Path, "from", from, "seq", chunks[i].Seq)
			}
			s.WriteTo(chunks[i].Data, from)
			chunks[i].Sent = now
			sent++
		}

		firstTime = false
	}
}

func ListenAndServe(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, db *sql.DB, handler front.Handler, resolver *fed.Resolver, addr string) error {
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
				if len(acks) < cfg.MaxSentGuppyChunks {
					acks <- pkt.Data
				}
				continue
			}
			if len(sessions) > cfg.MaxGuppySessions {
				log.Warn("Too many sessions")
				l.WriteTo([]byte("4 Too many sessions\r\n"), pkt.From)
				continue
			}

			acks := make(chan []byte, cfg.MaxSentGuppyChunks)
			sessions[k] = acks

			requestCtx, cancelRequest := context.WithTimeout(ctx, cfg.GuppyRequestTimeout)

			wg.Add(1)
			go func() {
				handle(requestCtx, domain, cfg, log, db, handler, resolver, &wg, pkt.From, pkt.Data, acks, done, l)
				cancelRequest()
				wg.Done()
			}()
		}
	}

	l.Close()
	wg.Wait()
	return nil
}
