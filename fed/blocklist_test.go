/*
Copyright 2024, 2025 Dima Krasner

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

package fed

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockList_NotBlockedDomain(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"0.0.0.0.com": {},
	}

	assert.False(blockList.Contains("127.0.0.1.com"))
}

func TestBlockList_BlockedDomain(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"0.0.0.0.com": {},
	}

	assert.True(blockList.Contains("0.0.0.0.com"))
}

func TestBlockList_BlockedSubdomain(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"social.0.0.0.0.com": {},
	}

	assert.True(blockList.Contains("social.0.0.0.0.com"))
}

func TestBlockList_NotBlockedSubdomain(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"social.0.0.0.0.com": {},
	}

	assert.False(blockList.Contains("blog.0.0.0.0.com"))
}

func TestBlockList_BlockedSubdomainByDomain(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"0.0.0.0.com": {},
	}

	assert.True(blockList.Contains("social.0.0.0.0.com"))
}

func TestBlockList_BlockedSubdomainByDomainEndsWithDot(t *testing.T) {
	assert := assert.New(t)

	blockList := BlockList{}
	blockList.domains = map[string]struct{}{
		"0.0.0.0.com": {},
	}

	assert.True(blockList.Contains("social.0.0.0.0.com."))
}
