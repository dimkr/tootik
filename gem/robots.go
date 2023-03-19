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
	"bytes"
	"context"
	"database/sql"
	"github.com/dimkr/tootik/data"
	"io"
	"net/url"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/robots.txt$`)] = robots
}

func robots(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	var buf bytes.Buffer
	buf.Write([]byte("20 text/gemini\r\n"))
	buf.Write([]byte("User-agent: *\n"))
	buf.Write([]byte("Disallow: /\n"))

	conn.Write(buf.Bytes())
}
