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

package gmi

import (
	"html"
	"regexp"
	"strings"
)

var (
	spanTags  = regexp.MustCompile(`(?:<span(?:\s+[^>]*)*>)+`)
	aTags     = regexp.MustCompile(`<a\s+(?:(?:[^>\s]+="[^"]*"\s+)*)href="([^"]*)"(?:\s*(?:\s+[^>\s]+="[^"]*")*\s*>)`)
	brTags    = regexp.MustCompile(`<(?:br\s*\/*|\/p)>`)
	openTags  = regexp.MustCompile(`(?:<[a-zA-Z0-9]+\s*[^>]*>)+`)
	closeTags = regexp.MustCompile(`(?:<\/[a-zA-Z0-9]+\s*[^>]*>)+`)
)

func FromHTML(text string) (string, []string) {
	res := html.UnescapeString(text)
	links := map[string]struct{}{}
	orderedLinks := []string{}

	for _, m := range brTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "\n\n", 1)
	}

	for _, m := range spanTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range aTags.FindAllStringSubmatch(res, -1) {
		link := m[1]
		if _, dup := links[link]; dup {
			continue
		}
		orderedLinks = append(orderedLinks, link)
		links[link] = struct{}{}
	}

	for _, m := range openTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range closeTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	return strings.TrimRight(res, " \n\r\t"), orderedLinks
}

func Quote(text string) string {
	if text == "" {
		return ""
	}

	res := "> " + strings.Join(strings.Split(text, "\n"), "\n> ")
	if res[len(res)-1] != '\n' {
		return res + "\n"
	}

	return res
}
