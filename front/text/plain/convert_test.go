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
	"maps"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func TestFromHTML_Empty(t *testing.T) {
	t.Parallel()

	post := ""
	expected := post
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Plain(t *testing.T) {
	t.Parallel()

	post := `this is a plain post`
	expected := post
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Paragraphs(t *testing.T) {
	t.Parallel()

	post := `<p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_TitleAndParagraphs(t *testing.T) {
	t.Parallel()

	post := `<h1>this is the title</h1><p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_TitleSubtitleAndParagraphs(t *testing.T) {
	t.Parallel()

	post := `<h1>this is the title</h1><h2>this is the subtitle</h2><p>this is a paragraph</p><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is the subtitle\n\nthis is a paragraph\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_TitleParagraphSubtitleAndParagraph(t *testing.T) {
	t.Parallel()

	post := `<h1>this is the title</h1><p>this is a paragraph</p><h2>this is the subtitle</h2><p>this is another paragraph</p>`
	expected := "this is the title\n\nthis is a paragraph\n\nthis is the subtitle\n\nthis is another paragraph"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_LineBreak(t *testing.T) {
	t.Parallel()

	post := `<p>this is a line<br/>this is another line</p>`
	expected := "this is a line\nthis is another line"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_MentionAndLink(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a>?`
	expected := "hi @x, have you seen c.d/e…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "c.d/e…")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_MentionAndLinks(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg"><span class="invisible">https://</span><span class="ellipsis">c.d/e</span><span class="invisible">fg</span></a> and <a href="https://h.i/jkl" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/jk</span><span class="invisible">l</span></a>?`
	expected := "hi @x, have you seen c.d/e… and h.i/jk…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "c.d/e…")
	expectedLinks.Store("https://h.i/jkl", "h.i/jk…")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_MentionAndLinkAltText(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <a href="https://c.d/efg">this <span>link</span></a>?`
	expected := "hi @x, have you seen this link?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg", "this link")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Mention(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, how are you?`
	expected := "hi @x, how are you?"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Image(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" />?`
	expected := "hi @x, have you seen https://c.d/efg.jpg?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_ImageAlt(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" />?`
	expected := "hi @x, have you seen this?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "this")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_ImageNoSrc(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img alt="this" />?`
	expected := "hi @x, have you seen this?"
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_ImageAndLink(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" /> and <a href="https://h.i/jkl" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/j</span><span class="invisible">kl</span></a>?`
	expected := "hi @x, have you seen this and h.i/j…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "this")
	expectedLinks.Store("https://h.i/jkl", "h.i/j…")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_ImageAndSameLink(t *testing.T) {
	t.Parallel()

	post := `hi <span class="h-card"><a href="https://a.b/@x" class="u-url mention">@<span>x</span></a></span>, have you seen <img src="https://c.d/efg.jpg" alt="this" /> and <a href="https://c.d/efg.jpg" target="_blank" rel="nofollow noopener noreferrer"><span class="invisible">https://</span><span class="ellipsis">h.i/j</span><span class="invisible">kl</span></a>?`
	expected := "hi @x, have you seen this and h.i/j…?"
	expectedLinks := data.OrderedMap[string, string]{}
	expectedLinks.Store("https://c.d/efg.jpg", "this")

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Escaping(t *testing.T) {
	t.Parallel()

	post := `<p>Things like &lt;p&gt; should be escaped</p>`
	expected := `Things like <p> should be escaped`
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_UnorderedList(t *testing.T) {
	t.Parallel()

	post := "<p>They said:</p><ul><li>First thing.</li><li>Second thing.</li></ul><p>And I disagree.</p>"
	expected := "They said:\n\n* First thing.\n* Second thing.\n\nAnd I disagree."
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_OrderedList(t *testing.T) {
	t.Parallel()

	post := "<p>They said:</p><ol><li>First thing.</li><li>Second thing.</li></ol><p>And I disagree.</p>"
	expected := "They said:\n\n1. First thing.\n2. Second thing.\n\nAnd I disagree."
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestFromHTML_Quote(t *testing.T) {
	t.Parallel()

	post := "<p>They said:</p><blockquote><p>First thing.<br/>Second thing.</p></blockquote><p>And I disagree.</p>"
	expected := "They said:\n\n> First thing.\n> Second thing.\n\nAnd I disagree."
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := FromHTML(post)

	if raw != expected {
		t.Fatalf("%s != %s", raw, expected)
	}

	if !maps.Equal(links, expectedLinks) {
		t.Fatalf("%v != %v", links, expectedLinks)
	}
}

func TestToHTML_Empty(t *testing.T) {
	t.Parallel()

	post := ``
	expected := post

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_Plain(t *testing.T) {
	t.Parallel()

	post := `this is a plain post`
	expected := `<p>this is a plain post</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LineBreak(t *testing.T) {
	t.Parallel()

	post := "this is a line\nthis is another line"
	expected := `<p>this is a line<br/>this is another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_TwoLineBreaks(t *testing.T) {
	t.Parallel()

	post := "this is a line\n\nthis is another line"
	expected := `<p>this is a line</p><p>this is another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_ManyLineBreaks(t *testing.T) {
	t.Parallel()

	post := "this is a line\n\n\n\n\n\n\nthis is another line"
	expected := `<p>this is a line</p><p>this is another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_ManyLinesManyLineBreaks(t *testing.T) {
	t.Parallel()

	post := "this is a line\n\n\n\n\n\n\nthis is another line\n\n\n\n\n\n\nthis is yet another line"
	expected := `<p>this is a line</p><p>this is another line</p><p>this is yet another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LeadingLineBreak(t *testing.T) {
	t.Parallel()

	post := "\nthis is a line\nthis is another line"
	expected := `<p><br/>this is a line<br/>this is another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LeadingLineBreaks(t *testing.T) {
	t.Parallel()

	post := "\n\n\nthis is a line\nthis is another line"
	expected := `<p><br/><br/><br/>this is a line<br/>this is another line</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_TrailingLineBreak(t *testing.T) {
	t.Parallel()

	post := "this is a line\nthis is another line\n"
	expected := `<p>this is a line<br/>this is another line<br/></p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_TrailingLineBreaks(t *testing.T) {
	t.Parallel()

	post := "this is a line\nthis is another line\n\n\n"
	expected := `<p>this is a line<br/>this is another line<br/><br/><br/></p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_OnlyLineBreaks(t *testing.T) {
	t.Parallel()

	post := "\n\n\n"
	expected := `<p><br/><br/><br/></p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_Link(t *testing.T) {
	t.Parallel()

	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh`
	expected := `<p>this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a></p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_OverlappingLink(t *testing.T) {
	t.Parallel()

	post := `this is a plain post with overlapping links: gemini://aa.bb.com/cc gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh`
	expected := `<p>this is a plain post with overlapping links: <a href="gemini://aa.bb.com/cc" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc</a> <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a></p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LinkAndLineBreak(t *testing.T) {
	t.Parallel()

	post := "this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh\n... and a line break"
	expected := `<p>this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a><br/>... and a line break</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LinkStart(t *testing.T) {
	t.Parallel()

	post := `gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh is a link`
	expected := `<p><a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a> is a link</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LinkDot(t *testing.T) {
	t.Parallel()

	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh.`
	expected := `<p>this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a>.</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_Question(t *testing.T) {
	t.Parallel()

	post := `have you seen gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh?`
	expected := `<p>have you seen <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a>?</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LinkExclamationMark(t *testing.T) {
	t.Parallel()

	post := `this is a plain post with a link: gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh!`
	expected := `<p>this is a plain post with a link: <a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a>!</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_LinkParentheses(t *testing.T) {
	t.Parallel()

	post := `this is a plain post with a link: (gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh)`
	expected := `<p>this is a plain post with a link: (<a href="gemini://aa.bb.com/cc?dd=ee&ff=gg%20hh" target="_blank" rel="nofollow noopener noreferrer">gemini://aa.bb.com/cc?dd=ee&amp;ff=gg%20hh</a>)</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_TitleAndParagraphs(t *testing.T) {
	t.Parallel()

	post := "this is the title\n\nthis is a paragraph\n\nthis is another paragraph"
	expected := `<p>this is the title</p><p>this is a paragraph</p><p>this is another paragraph</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_TitleSubtitleAndParagraphs(t *testing.T) {
	t.Parallel()

	post := "this is the title\n\nthis is the subtitle\n\nthis is a paragraph\n\nthis is another paragraph"
	expected := `<p>this is the title</p><p>this is the subtitle</p><p>this is a paragraph</p><p>this is another paragraph</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_Mentions(t *testing.T) {
	t.Parallel()

	post := "hi @alice, @bob, @alice@localhost.localdomain:8443 and @alice, how are you?"
	expected := `<p>hi <span class="h-card" translate="no"><a href="https://localhost.localdomain:8443/user/alice" class="u-url mention">@alice</a></span>, @bob, <span class="h-card" translate="no"><a href="https://localhost.localdomain:8443/user/alice" class="u-url mention">@alice@localhost.localdomain:8443</a></span> and @alice, how are you?</p>`
	mentions := []ap.Tag{
		{
			Type: ap.Mention,
			Name: "@alice",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
		{
			Type: ap.Mention,
			Name: "@alice@localhost.localdomain:8443",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
	}

	if html := ToHTML(post, mentions); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_MissingMentions(t *testing.T) {
	t.Parallel()

	post := "hi alice, bob, alice@localhost.localdomain:8443 and @alice, how are you?"
	expected := `<p>hi alice, bob, alice@localhost.localdomain:8443 and <span class="h-card" translate="no"><a href="https://localhost.localdomain:8443/user/alice" class="u-url mention">@alice</a></span>, how are you?</p>`
	mentions := []ap.Tag{
		{
			Type: ap.Mention,
			Name: "@alice",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
	}

	if html := ToHTML(post, mentions); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_NoMentions(t *testing.T) {
	t.Parallel()

	post := "hi alice, bob, alice@localhost.localdomain:8443 and alice, how are you?"
	expected := `<p>hi alice, bob, alice@localhost.localdomain:8443 and alice, how are you?</p>`
	mentions := []ap.Tag{
		{
			Type: ap.Mention,
			Name: "@alice",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
		{
			Type: ap.Mention,
			Name: "@alice@localhost.localdomain:8443",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
	}

	if html := ToHTML(post, mentions); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_EmojiMention(t *testing.T) {
	t.Parallel()

	post := "hi @alice, @bob, @alice@localhost.localdomain:8443 and @alice, how are you?"
	expected := `<p>hi <span class="h-card" translate="no"><a href="https://localhost.localdomain:8443/user/alice" class="u-url mention">@alice</a></span>, @bob, <span class="h-card" translate="no"><a href="https://localhost.localdomain:8443/user/alice" class="u-url mention">@alice@localhost.localdomain:8443</a></span> and @alice, how are you?</p>`
	mentions := []ap.Tag{
		{
			Type: ap.Mention,
			Name: "@alice",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
		{
			Type: ap.Emoji,
			Name: "@bob",
			Href: "https://localhost.localdomain:8443/user/bob",
		},
		{
			Type: ap.Mention,
			Name: "@alice@localhost.localdomain:8443",
			Href: "https://localhost.localdomain:8443/user/alice",
		},
	}

	if html := ToHTML(post, mentions); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}

func TestToHTML_Escaping(t *testing.T) {
	t.Parallel()

	post := `Things like <p> should be escaped`
	expected := `<p>Things like &lt;p&gt; should be escaped</p>`

	if html := ToHTML(post, nil); html != expected {
		t.Fatalf("%s != %s", html, expected)
	}
}
