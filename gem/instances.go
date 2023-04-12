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

package gem

import (
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"io"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/users/instances$`)] = withCache(withUserMenu(instances), time.Hour)
	handlers[regexp.MustCompile(`^/instances$`)] = withCache(withUserMenu(instances), time.Hour)
}

func instances(w io.Writer, r *request) {
	instances, err := r.Query(`select distinct substr(substr(author, 9), 1, instr(substr(author, 9), '/')-1) from notes order by inserted desc`)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to list known instances")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	defer instances.Close()

	w.Write([]byte("20 text/gemini\r\n"))
	w.Write([]byte("# 🌕 Other Servers\n\n"))

	for instances.Next() {
		var host string
		if err := instances.Scan(&host); err != nil {
			r.Log.WithError(err).Warn("Failed to fetch a host")
			continue
		}

		if host != cfg.Domain {
			fmt.Fprintf(w, "=> https://%s %s\n", host, host)
		}
	}
}
