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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFromHTML_Empty(t *testing.T) {
	post := ""
	expected := ""
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Empty(t, links)
}

func TestFromHTML_Plain(t *testing.T) {
	post := `this is a plain post`
	raw, links := FromHTML(post)
	assert.Equal(t, post, raw)
	assert.Empty(t, links)
}

func TestFromHTML_Paragraphs(t *testing.T) {
	post := `<p>this is a paragraph</p><p>this another paragraph</p>`
	expected := "this is a paragraph\n\nthis another paragraph"
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Empty(t, links)
}

func TestFromHTML_LineBreak(t *testing.T) {
	post := `<p>this is a line<br/>this another line</p>`
	expected := "this is a line\n\nthis another line"
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Empty(t, links)
}

func TestFromHTML_MentionAndLink(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a>?`
	expected := "hi @x, have you seen https://c.d/efg?"
	expectedLinks := []string{"https://c.d/efg"}
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_MentionAndLinks(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a> and <a href="https://h.i/jkl" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/jk</span><span class="invisible">l</span></a>?`
	expected := "hi @x, have you seen https://c.d/efg and https://h.i/jkl?"
	expectedLinks := []string{"https://c.d/efg", "https://h.i/jkl"}
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_MentionAndLinkAltText(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer"><span class="ellipsis">this</span> <span>link</span></a>?`
	expected := "hi @x, have you seen this link?"
	expectedLinks := []string{"https://c.d/efg"}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_Mention(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, how are you?`
	expected := "hi @x, how are you?"
	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Empty(t, links)
}
