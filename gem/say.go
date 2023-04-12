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
	"github.com/dimkr/tootik/ap"
	"io"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/say$`)] = say
}

func say(w io.Writer, r *request) {
	to := ap.Audience{}
	cc := ap.Audience{}

	to.Add(ap.Public)

	post(w, r, nil, to, cc)
}
