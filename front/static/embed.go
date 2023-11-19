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

package static

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.gmi */*.gmi
var rawFiles embed.FS

var Files = map[string][]string{}

func readDirectory(dir string) {
	files, err := rawFiles.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.IsDir() {
			readDirectory(file.Name())
			continue
		}

		name := file.Name()

		path := name
		if dir != "." {
			path = fmt.Sprintf("%s/%s", dir, path)
		}

		content, err := rawFiles.ReadFile(path)
		if err != nil {
			panic(err)
		}

		base := name
		if dot := strings.LastIndexByte(name, '.'); dot > 0 {
			base = base[:dot]
		}

		if dir == "." {
			Files[fmt.Sprintf("/%s", base)] = strings.Split(strings.TrimRight(string(content), "\r\n\t "), "\n")
		} else {
			Files[fmt.Sprintf("/%s/%s", dir, base)] = strings.Split(strings.TrimRight(string(content), "\r\n\t "), "\n")
		}
	}
}

func init() {
	readDirectory(".")
}
