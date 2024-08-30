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

package gmap

import (
	"bytes"
	"github.com/dimkr/tootik/cfg"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRaw_TrailingNewLine(t *testing.T) {
	assert := assert.New(t)

	var b bytes.Buffer
	w := Wrap(&b, "localhost.localdomain:8443", &cfg.Config{LineWidth: 70})

	w.Raw(
		"Alt text",
		"  _\n\\'_'/\n|___|\n/   \\\n",
	)
	w.Flush()

	assert.Equal(
		"i  _\t/\t0\t0\r\ni\\'_'/\t/\t0\t0\r\ni|___|\t/\t0\t0\r\ni/   \\\t/\t0\t0\r\n",
		b.String(),
	)
}

func TestRaw_NoTrailingNewLine(t *testing.T) {
	assert := assert.New(t)

	var b bytes.Buffer
	w := Wrap(&b, "localhost.localdomain:8443", &cfg.Config{LineWidth: 70})

	w.Raw(
		"Alt text",
		"  _\n\\'_'/\n|___|\n/   \\",
	)
	w.Flush()

	assert.Equal(
		"i  _\t/\t0\t0\r\ni\\'_'/\t/\t0\t0\r\ni|___|\t/\t0\t0\r\ni/   \\\t/\t0\t0\r\n",
		b.String(),
	)
}
