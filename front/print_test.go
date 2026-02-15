/*
Copyright 2023 - 2026 Dima Krasner

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

package front

import (
	"testing"

	"github.com/dimkr/tootik/data"
	"github.com/stretchr/testify/assert"
)

func TestGetTextAndLinks_EmptyPost(t *testing.T) {
	post := ``
	expected := []string{
		"[no content]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_FewLines(t *testing.T) {
	post := `<p>this is line 1</p><p>this is line 2</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLines(t *testing.T) {
	post := `<p>this is line 1</p><p>this is line 2</p><p>this is line 3</p><p>this is line 4</p><p>this is line 5</p><p>this is line 6</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
		"[…]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesExtraLineBreak(t *testing.T) {
	post := `<p>this is line 1</p><br><p>this is line 2</p><p>this is line 3</p><p>this is line 4</p><p>this is line 5</p><p>this is line 6</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
		"[…]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesExtraLineBreaks(t *testing.T) {
	post := `<p>this is line 1</p><br><br><p>this is line 2</p><p>this is line 3</p><p>this is line 4</p><p>this is line 5</p><p>this is line 6</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
		"[…]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesNoParagraphs(t *testing.T) {
	post := `this is line 1<br/>this is line 2<br/>this is line 3<br/>this is line 4<br/>this is line 5<br/>this is line 6`
	expected := []string{
		"this is line 1",
		"this is line 2",
		"this is line 3",
		"[…]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesNoParagraphsExtraLineBreak(t *testing.T) {
	post := `this is line 1<br/>this is line 2<br/><br/>this is line 3<br/>this is line 4<br/>this is line 5<br/>this is line 6`
	expected := []string{
		"this is line 1",
		"this is line 2",
		"[…]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesManyRunes(t *testing.T) {
	post := `<p>this is line 1</p><p>this is line 2</p><p>this is line 3</p><p>this is line 4</p><p>this is line 5</p><p>this is line 6</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line […]",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200/5-2, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_EmptyLinesInMiddle(t *testing.T) {
	post := `this is line 1<br><br><br><br><br><br><br><br>this is line 2`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_EmptyLinesInMiddleThenMoreLines(t *testing.T) {
	post := `this is line 1<br><br><br><br><br><br><br><br>this is line 2<br>this is line 3`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
		"this is line 3",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, 4)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}

func TestGetTextAndLinks_ManyLinesNoLinesLimit(t *testing.T) {
	post := `<p>this is line 1</p><p>this is line 2</p><p>this is line 3</p><p>this is line 4</p><p>this is line 5</p><p>this is line 6</p>`
	expected := []string{
		"this is line 1",
		"",
		"this is line 2",
		"",
		"this is line 3",
		"",
		"this is line 4",
		"",
		"this is line 5",
		"",
		"this is line 6",
	}
	expectedLinks := data.OrderedMap[string, string]{}

	raw, links := getTextAndLinks(post, 200, -1)
	assert.Equal(t, expected, raw)
	assert.Equal(t, expectedLinks, links)
}
