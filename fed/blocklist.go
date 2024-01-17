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

package fed

import (
	"encoding/csv"
	"github.com/fsnotify/fsnotify"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BlockList is a list of blocked domains.
type BlockList struct {
	lock    sync.Mutex
	wg      sync.WaitGroup
	w       *fsnotify.Watcher
	domains map[string]struct{}
}

const blockListReloadDelay = time.Second * 5

func loadBlocklist(path string) (map[string]struct{}, error) {
	blockedDomains := make(map[string]struct{})

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := csv.NewReader(f)
	first := true
	for {
		r, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if first {
			first = false
			continue
		}

		blockedDomains[r[0]] = struct{}{}
	}

	return blockedDomains, nil
}

func NewBlockList(log *slog.Logger, path string) (*BlockList, error) {
	domains, err := loadBlocklist(path)
	if err != nil {
		return nil, err
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(path)
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}
	absPath := filepath.Join(dir, filepath.Base(path))

	b := &BlockList{w: w, domains: domains}

	timer := time.NewTimer(math.MaxInt64)
	timer.Stop()

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					timer.Stop()
					return
				}

				if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && event.Name == absPath {
					timer.Reset(blockListReloadDelay)
				}

			case <-timer.C:
				newDomains, err := loadBlocklist(path)
				if err != nil {
					log.Warn("Failed to reload blocklist", "path", path, "error", err)
					continue
				}

				// continue if the old list wasn't empty and the new one is empty; maybe the file was opened with O_TRUNC
				if len(b.domains) > 0 && len(newDomains) == 0 {
					log.Warn("New blocklist is empty")
					continue
				}

				b.lock.Lock()
				b.domains = newDomains
				b.lock.Unlock()
				log.Info("Reloaded blocklist", "path", path, "length", len(newDomains))
			}
		}
	}()

	return b, nil
}

// Contains determines if a domain is blocked.
func (b *BlockList) Contains(domain string) bool {
	b.lock.Lock()
	_, contains := b.domains[domain]
	b.lock.Unlock()
	return contains
}

// Close frees resources.
func (b *BlockList) Close() {
	b.w.Close()
	b.wg.Wait()
}
