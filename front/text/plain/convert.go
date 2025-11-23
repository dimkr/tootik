/*
Copyright 2023 - 2025 Dima Krasner

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
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	tokenizer "golang.org/x/net/html"
)

const maxDepth = 32

var (
	urlRegex                = regexp.MustCompile(`\b(https|http|gemini|titan|gopher|gophers|spartan|guppy):\/\/\S+\b`)
	pDelim                  = regexp.MustCompile(`([^\n])\n\n+([^\n])`)
	mentionRegex            = regexp.MustCompile(`\B@(\w+)(?:@(?:(?:\w+\.)+\w+(?::\d{1,5}){0,1})){0,1}\b`)
	multipleLineBreaksRegex = regexp.MustCompile(`\n{3,}`)
)

func fromHTML(text string) (string, data.OrderedMap[string, string], error) {
	links := data.OrderedMap[string, string]{}

	tok := tokenizer.NewTokenizer(strings.NewReader(text))

	var (
		b           strings.Builder
		openTags    []string
		linkText    strings.Builder
		currentLink string
	)
	invisibleDepth := 0
	ellipsisDepth := 0
	w := &b
	inLink := false
	inUl := false
	inOl := false
	quoteDepth := 0
	olIndex := 0

	for {
		tt := tok.Next()
		switch tt {
		case tokenizer.ErrorToken:
			err := tok.Err()

			if errors.Is(err, io.EOF) {
				return strings.TrimRight(multipleLineBreaksRegex.ReplaceAllLiteralString(b.String(), "\n\n"), " \n\r\t"), links, nil
			}

			return "", nil, err

		case tokenizer.TextToken:
			if invisibleDepth > 0 {
				continue
			}

			if quoteDepth > 0 {
				l := w.Len()
				if l > 0 && w.String()[l-1] == '\n' {
					for range quoteDepth {
						w.WriteString("> ")
					}
				}
			}

			w.Write(tok.Text())

		case tokenizer.EndTagToken:
			tagBytes, _ := tok.TagName()
			tag := danger.String(tagBytes)

			if len(openTags) > 0 && tag == openTags[len(openTags)-1] {
				openTags = openTags[:len(openTags)-1]
			} else {
				return "", nil, fmt.Errorf("tag not opened: %s", tag)
			}

			if tag == "p" || (len(tag) == 2 && tag[0] == 'h' && tag[1] > '0' && tag[1] <= '9') {
				w.WriteString("\n\n")
				continue
			}

			if tag == "a" {
				if currentLink != "" {
					alt := linkText.String()

					if !links.Contains(currentLink) {
						links.Store(currentLink, alt)
					}

					b.WriteString(alt)
					linkText.Reset()
					currentLink = ""
					w = &b
				}

				inLink = false
			} else if inUl && tag == "ul" {
				w.WriteString("\n\n")
				inUl = false
			} else if inOl && tag == "ol" {
				w.WriteString("\n\n")
				inOl = false
			} else if tag == "blockquote" {
				quoteDepth--
			}

			if len(openTags)+1 == ellipsisDepth {
				if invisibleDepth == 0 {
					w.WriteRune('…')
				}

				ellipsisDepth = 0
			}

			if len(openTags)+1 == invisibleDepth {
				invisibleDepth = 0
			}

		case tokenizer.StartTagToken, tokenizer.SelfClosingTagToken:
			tagBytes, hasAttrs := tok.TagName()
			tag := danger.String(tagBytes)

			if tag == "br" {
				w.WriteByte('\n')
				continue
			}

			if tt == tokenizer.StartTagToken && tag != "img" {
				if len(openTags) == maxDepth {
					return "", nil, errors.New("too nested")
				}

				openTags = append(openTags, tag)
			}

			if tag == "ul" {
				if inUl || inOl {
					return "", nil, errors.New("lists cannot be nested")
				}

				inUl = true
				continue
			}

			if tag == "ol" {
				if inUl || inOl {
					return "", nil, errors.New("lists cannot be nested")
				}

				inOl = true
				olIndex = 0
				continue
			}

			if tag == "li" {
				if !inUl && !inOl {
					return "", nil, errors.New("list item outside of a list")
				}

				l := w.Len()
				if !(l > 0 && w.String()[l-1] == '\n') {
					w.WriteByte('\n')
				}

				if inOl {
					olIndex++
					w.WriteString(strconv.Itoa(olIndex))
					w.WriteString(". ")
				} else {
					w.WriteString("* ")
				}
			}

			if tag == "blockquote" {
				quoteDepth++
				continue
			}

			var alt, src, class, href string
			if hasAttrs {
				for {
					attrBytes, value, more := tok.TagAttr()

					attr := danger.String(attrBytes)
					valueString := danger.String(value)
					if tt == tokenizer.StartTagToken && tag == "span" && attr == "class" {
						if valueString == "invisible" {
							invisibleDepth = len(openTags)
						} else if valueString == "ellipsis" {
							ellipsisDepth = len(openTags)
						}
					} else if tag == "a" && attr == "class" {
						class = valueString
					} else if tag == "a" && attr == "href" {
						href = valueString
					} else if tag == "img" && attr == "alt" {
						alt = valueString
					} else if tag == "img" && attr == "src" {
						src = valueString
					}

					if !more {
						break
					}
				}
			}

			if tag == "a" {
				if inLink {
					return "", nil, errors.New("links cannot be nested")
				}

				if href != "" && class != "mention" && !strings.HasPrefix(class, "mention ") && !strings.HasSuffix(class, " mention") {
					currentLink = href
					w = &linkText
					continue
				}

				inLink = true
			}

			if alt != "" {
				w.WriteString(alt)
			} else if src != "" {
				w.WriteString(src)
			}

			if src != "" {
				if !links.Contains(src) {
					links.Store(src, alt)
				}
			}
		}
	}
}

// FromHTML converts HTML to plain text and extracts links.
func FromHTML(text string) (string, data.OrderedMap[string, string]) {
	plain, links, err := fromHTML(text)
	if err != nil {
		slog.Warn("Failed to convert post", "error", err)
		return text, data.OrderedMap[string, string]{}
	}

	return plain, links
}

// ToHTML converts plain text to HTML.
func ToHTML(text string, tags []ap.Tag) string {
	if text == "" {
		return ""
	}

	text = html.EscapeString(text)

	var b strings.Builder

	foundLink := false
	for {
		loc := urlRegex.FindStringIndex(text)
		if loc == nil {
			break
		}
		b.WriteString(text[:loc[0]])
		b.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" rel="nofollow noopener noreferrer">%s</a>`, html.UnescapeString(text[loc[0]:loc[1]]), text[loc[0]:loc[1]]))
		text = text[loc[1]:]
		foundLink = true
	}
	if foundLink {
		b.WriteString(text)
		text = b.String()
	}

	if len(tags) > 0 {
		b.Reset()
	mentions:
		for _, tag := range tags {
			if tag.Type != ap.Mention {
				continue
			}
			for {
				loc := mentionRegex.FindStringSubmatchIndex(text)
				if loc == nil {
					break mentions
				}
				b.WriteString(text[:loc[0]])
				if text[loc[0]:loc[1]] == tag.Name {
					b.WriteString(fmt.Sprintf(`<span class="h-card" translate="no"><a href="%s" class="u-url mention">%s</a></span>`, tag.Href, text[loc[0]:loc[1]]))
					text = text[loc[1]:]
					break
				}

				b.WriteString(text[loc[0]:loc[1]])
				text = text[loc[1]:]
			}
		}
		b.WriteString(text)

		text = b.String()
	}

	text = pDelim.ReplaceAllString(text, "$1</p><p>$2")
	text = strings.ReplaceAll(text, "\n", "<br/>")
	return fmt.Sprintf("<p>%s</p>", text)
}
