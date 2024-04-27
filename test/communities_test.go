/*
Copyright 2024 Dima Krasner

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

package test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestCommunities_OneCommunity(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = json_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	communities := server.Handle("/users/communities", server.Bob)
	assert.Contains(strings.Split(communities, "\n"), fmt.Sprintf("=> /users/outbox/%s/user/alice %s alice", domain, time.Now().Format(time.DateOnly)))
}
