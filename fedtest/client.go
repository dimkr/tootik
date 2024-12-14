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

package fedtest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// Client is a [fed.Client] that handles a HTTP request by calling the destination server's [http.Handler].
type Client map[string]*Server

type responseWriter struct {
	StatusCode int
	Headers    http.Header
	Body       bytes.Buffer
}

func (w *responseWriter) Header() http.Header {
	return w.Headers
}

func (w *responseWriter) Write(buf []byte) (int, error) {
	if w.StatusCode == 0 {
		w.StatusCode = http.StatusOK
	}

	return w.Body.Write(buf)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}

func (f Client) Do(r *http.Request) (*http.Response, error) {
	dst := f[r.URL.Host]
	if dst == nil {
		return nil, fmt.Errorf("Unknown server: %s", r.URL.Host)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	dst.Backend.ServeHTTP(&w, r)

	return &http.Response{
		StatusCode: w.StatusCode,
		Header:     w.Headers,
		Body:       io.NopCloser(bytes.NewReader(w.Body.Bytes())),
	}, nil
}
