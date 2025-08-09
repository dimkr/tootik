/*
Copyright 2024, 2025 Dima Krasner

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

package httpsig

import (
	"errors"
	"net/http"
	"net/textproto"
	"strings"
)

func buildSignatureString(r *http.Request, headers []string) (string, error) {
	var b strings.Builder

	for i, h := range headers {
		switch h {
		case "(request-target)":
			b.WriteString("(request-target)")
			b.WriteByte(':')
			b.WriteByte(' ')
			b.WriteString(strings.ToLower(r.Method))
			b.WriteByte(' ')
			b.WriteString(r.URL.Path)

		default:
			if h[0] == '(' {
				return "", errors.New("unsupported header: " + h)
			}
			b.WriteString(strings.ToLower(h))
			b.WriteByte(':')
			b.WriteByte(' ')
			values, ok := r.Header[textproto.CanonicalMIMEHeaderKey(h)]
			if !ok || len(values) == 0 {
				return "", errors.New("unspecified header: " + h)
			}
			for j, v := range values {
				b.WriteString(strings.TrimSpace(v))
				if j < len(values)-1 {
					b.WriteByte(',')
					b.WriteByte(' ')
				}
			}
		}

		if i < len(headers)-1 {
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}
