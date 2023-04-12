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
	"io"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/oops`)] = withUserMenu(oops)
	handlers[regexp.MustCompile(`^/oops`)] = withUserMenu(oops)
}

func oops(w io.Writer, r *request) {
	w.Write([]byte("20 text/gemini\r\n"))
	w.Write([]byte("# 🦖🦖🦖\n"))
	w.Write([]byte{'\n'})
	w.Write([]byte("You sent an invalid request or this page doesn't exist.\n"))
}
