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
	"github.com/dimkr/tootik/data"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFromHTML_Empty(t *testing.T) {
	post := ""
	expected := post
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_Plain(t *testing.T) {
	post := `this is a plain post`
	expected := post
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_Paragraphs(t *testing.T) {
	post := `<p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_TitleAndParagraphs(t *testing.T) {
	post := `<h1>this is the title</h1><p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_TitleSubtitleAndParagraphs(t *testing.T) {
	post := `<h1>this is the title</h1><h2>this is the subtitle</h2><p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is the subtitle\n\nthis is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_TitleParagraphSubtitleAndParagraph(t *testing.T) {
	post := `<h1>this is the title</h1><p>this is a paragraph</p><h2>this is the subtitle</h2><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is a paragraph\n\nthis is the subtitle\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_LineBreak(t *testing.T) {
	post := `<p>this is a line<br/>this is another line</p>`
	expected := "this is a line\nthis is another line"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_MentionAndLink(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a>?`
	expected := "hi @x, have you seen c.d/e…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_MentionAndLinks(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a> and <a href="https://h.i/jkl" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/jk</span><span class="invisible">l</span></a>?`
	expected := "hi @x, have you seen c.d/e… and h.i/jk…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "")
	expectedLinks.Store("https://h.i/jkl", "")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_MentionAndLinkAltText(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg" target="_blank" rel="nofollow noopener noreferrer">this <span>link</span></a>?`
	expected := "hi @x, have you seen this link?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "")

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

func TestToHTML_Plain(t *testing.T) {
	post := `this is a plain post`
	expected := post

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LineBreak(t *testing.T) {
	post := "this is a line\nthis is another line"
	expected := `<p>this is a line</p><p>this is another line</p>`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_Link(t *testing.T) {
	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh`
	expected := `this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a>`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LinkAndLineBreak(t *testing.T) {
	post := "this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh\n... and a line break"
	expected := `<p>this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a></p><p>... and a line break</p>`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LinkStart(t *testing.T) {
	post := `gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh is a link`
	expected := `<a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a> is a link`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LinkDot(t *testing.T) {
	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh.`
	expected := `this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a>.`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_Question(t *testing.T) {
	post := `have you seen gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh?`
	expected := `have you seen <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a>?`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LinkExclamationMark(t *testing.T) {
	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh!`
	expected := `this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a>!`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestToHTML_LinkParentheses(t *testing.T) {
	post := `this is a plain post with a link: (gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh)`
	expected := `this is a plain post with a link: (<a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank">gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh</a>)`

	html := ToHTML(post)
	assert.Equal(t, expected, html)
}

func TestFromHTML_Image(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" />?`
	expected := "hi @x, have you seen [https://c.d/efg.jpg]?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_ImageAlt(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" />?`
	expected := "hi @x, have you seen [this]?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "this")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_ImageNoSrc(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img alt="this" />?`
	expected := "hi @x, have you seen [this]?"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_ImageAndLink(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" /> and <a href="https://h.i/jkl" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/j</span><span class="invisible">kl</span></a>?`
	expected := "hi @x, have you seen [this] and h.i/j…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://h.i/jkl", "")
	expectedLinks.Store("https://c.d/efg.jpg", "this")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestFromHTML_ImageAndSameLink(t *testing.T) {
	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" /> and <a href="https://c.d/efg.jpg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/j</span><span class="invisible">kl</span></a>?`
	expected := "hi @x, have you seen [this] and h.i/j…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "")

	raw, links := FromHTML(post)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}
