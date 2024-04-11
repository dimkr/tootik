/*
Copyright 2024 Dima Krasner

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

package front

import (
	"github.com/dimkr/tootik/front/text"
	"io"
	"net/url"
	"strconv"
)

// inputFunc is a callback that returns user-provided text or false
type inputFunc func() (string, bool)

func readQuery(w text.Writer, r *request, prompt string) (string, bool) {
	if r.URL.RawQuery == "" {
		w.Status(10, prompt)
		return "", false
	}

	content, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		w.Status(40, "Bad input")
		return "", false
	}

	return content, true
}

func readBody(w text.Writer, r *request, args []string) (string, bool) {
	if r.Body == nil {
		w.Redirect("/users/oops")
		return "", false
	}

	var sizeStr, mimeType string
	if args[1] == "size" && args[3] == "mime" {
		sizeStr = args[2]
		mimeType = args[4]
	} else if args[1] == "mime" && args[3] == "size" {
		sizeStr = args[4]
		mimeType = args[2]
	} else {
		r.Log.Warn("Invalid parameters")
		w.Status(40, "Invalid parameters")
		return "", false
	}

	if mimeType != "text/plain" {
		r.Log.Warn("Content type is unsupported", "type", mimeType)
		w.Status(40, "Only text/plain is supported")
		return "", false
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse content size", "error", err)
		w.Status(40, "Invalid size")
		return "", false
	}

	if size == 0 {
		r.Log.Warn("Content is empty")
		w.Status(40, "Content is empty")
		return "", false
	}

	if size > int64(r.Handler.Config.MaxPostsLength)*4 {
		r.Log.Warn("Content is too big", "size", size)
		w.Status(40, "Content is too big")
		return "", false
	}

	buf := make([]byte, size)
	n, err := io.ReadFull(r.Body, buf)
	if err != nil {
		r.Log.Warn("Failed to read content", "error", err)
		w.Error()
		return "", false
	}

	if int64(n) != size {
		r.Log.Warn("Content is truncated")
		w.Error()
		return "", false
	}

	return string(buf), true
}
