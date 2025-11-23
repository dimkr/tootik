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

// Package static serves static content.
package static

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/danger"
)

type data struct {
	Domain string
	Config *cfg.Config
}

//go:embed *.gmi */*.gmi
var vfs embed.FS

var templates = map[string]*template.Template{}

func Format(domain string, cfg *cfg.Config) (map[string][]string, error) {
	formatted := make(map[string][]string, len(templates))

	data := data{
		Domain: domain,
		Config: cfg,
	}

	for path, tmpl := range templates {
		var b bytes.Buffer
		if err := tmpl.Execute(&b, &data); err != nil {
			return nil, err
		}

		formatted[path] = strings.Split(strings.TrimRight(b.String(), "\r\n\t "), "\n")
	}

	return formatted, nil
}

func readDirectory(dir string) {
	files, err := vfs.ReadDir(dir)
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

		content, err := vfs.ReadFile(path)
		if err != nil {
			panic(err)
		}

		base := name
		if dot := strings.LastIndexByte(name, '.'); dot > 0 {
			base = base[:dot]
		}

		if dir == "." {
			path = fmt.Sprintf("/%s", base)
		} else {
			path = fmt.Sprintf("/%s/%s", dir, base)
		}

		tmpl, err := template.New(path).Parse(danger.String(content))
		if err != nil {
			panic(err)
		}

		templates[path] = tmpl
	}
}

func init() {
	readDirectory(".")
}
