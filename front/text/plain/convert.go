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

package plain

import (
	"fmt"
	"github.com/dimkr/tootik/data"
	"html"
	"regexp"
	"strings"
)

var (
	spanTags          = regexp.MustCompile(`(?:<span(?:\s+[^>]*)*>)+`)
	aTags             = regexp.MustCompile(`<a\s+(?:(?:[^>\s]+="[^"]*"\s+)*)href="([^"]*)"(?:\s*(?:\s+[^>\s]+="[^"]*")*\s*>)`)
	imgTags           = regexp.MustCompile(`<img(?:\s+([a-z]+="[^"]*"))+\s*\/*>`)
	attrs             = regexp.MustCompile(`\s+([a-z]+)="([^"]*)"`)
	mentionTags       = regexp.MustCompile(`<a\s+(?:[^\s<]+\s+)*class="(?:[^\s"]+\s+)*mention(?:\s+[^\s"]+)*"[^>]*>`)
	invisibleSpanTags = regexp.MustCompile(`<span class="invisible">[^<]*</span>`)
	ellipsisSpanTags  = regexp.MustCompile(`<span class="ellipsis">[^<]*</span>`)
	pTags             = regexp.MustCompile(`<(?:/p|\/h\d+)>`)
	brTags            = regexp.MustCompile(`<br\s*\/*>`)
	openTags          = regexp.MustCompile(`(?:<[a-zA-Z0-9]+\s*[^>]*>)+`)
	closeTags         = regexp.MustCompile(`(?:<\/[a-zA-Z0-9]+\s*[^>]*>)+`)
	urlRegex          = regexp.MustCompile(`\b(https|http|gemini|gopher|gophers):\/\/\S+\b`)
)

func FromHTML(text string) (string, data.OrderedMap[string, string]) {
	res := html.UnescapeString(text)
	links := data.OrderedMap[string, string]{}

	for _, m := range mentionTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range pTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "\n\n", 1)
	}

	for _, m := range brTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "\n", 1)
	}

	for _, m := range invisibleSpanTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range ellipsisSpanTags.FindAllStringSubmatch(res, -1) {
		res = strings.Replace(res, m[0], m[0]+"…", 1)
	}

	for _, m := range spanTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range aTags.FindAllStringSubmatch(res, -1) {
		link := m[1]
		if !links.Contains(link) {
			links.Store(link, "")
		}
	}

	for _, img := range imgTags.FindAllStringSubmatch(res, -1) {
		var alt, src string
		for _, attr := range attrs.FindAllStringSubmatch(img[0], -1) {
			if attr[1] == "alt" {
				alt = attr[2]
				if src != "" {
					break
				}
			} else if attr[1] == "src" {
				src = attr[2]
				if alt != "" {
					break
				}
			}
		}

		if alt != "" {
			res = strings.Replace(res, img[0], "["+alt+"]", 1)
		} else if src != "" {
			res = strings.Replace(res, img[0], "["+src+"]", 1)
		}

		if src != "" {
			if !links.Contains(src) {
				links.Store(src, alt)
			}
		}
	}

	for _, m := range openTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	for _, m := range closeTags.FindAllString(res, -1) {
		res = strings.Replace(res, m, "", 1)
	}

	return strings.TrimRight(res, " \n\r\t"), links
}

func getPlainLinks(text string) map[string]struct{} {
	links := map[string]struct{}{}
	for _, link := range urlRegex.FindAllString(text, -1) {
		links[link] = struct{}{}
	}
	return links
}

func ToHTML(text string) string {
	for link := range getPlainLinks(text) {
		text = strings.ReplaceAll(text, link, fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, link, link))
	}

	text = regexp.MustCompile(`([^\n]\n*)\n\n([^\n])`).ReplaceAllString(text, "$1</p><p>$2")
	text = strings.ReplaceAll(text, "\n", "<br/>")
	return "<p>" + text + "</p>"
}
