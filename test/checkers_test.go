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
)

func TestCheckers_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/644253", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/513342", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/556400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/334400", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/355344", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/624453", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/061500", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/223300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/042213", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/311322", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/465500", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/132400", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/263500", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/240615", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/355344", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/332400", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/536200", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/715362", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/537564", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/574600", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/755766", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/573546", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/776600", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/241500", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/556400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/152600", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/371526", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/062415", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/665500", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/241500", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/554400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/355344", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/537564", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/170600", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/755700", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/062415", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/570224", server.Bob))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "bob won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "You won.")

	assert.Regexp(`^40 Please wait for \S+\r\n$`, server.Handle("/users/checkers/start", server.Alice))
}

func TestCheckers_StartTwice(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/start", server.Alice))
}

func TestCheckers_StartSurrenderStart(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MinCheckersInterval = 0

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/start", server.Alice))
}

func TestCheckers_SelfJoin(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("40 Already joined\r\n", server.Handle("/users/checkers/join/1", server.Alice))
}

func TestCheckers_AlreadyJoined(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("40 Already joined\r\n", server.Handle("/users/checkers/join/1", server.Bob))
}

func TestCheckers_AlreadyJoinedAnotherGame(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/start", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Carol))
	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/join/2", server.Carol))
}

func TestCheckers_JoinStart(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/start", server.Bob))
}

func TestCheckers_JoinSurrenderJoin(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MinCheckersInterval = 0

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/start", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Carol))
	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/join/2", server.Carol))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Carol))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/join/2", server.Carol))
}

func TestCheckers_JoinSurrenderStart(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MinCheckersInterval = 0

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/start", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Carol))
	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/join/2", server.Carol))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Carol))
	assert.Equal("30 /users/checkers/3\r\n", server.Handle("/users/checkers/start", server.Carol))
}

func TestCheckers_HumanSurrenderedJoin(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MinCheckersInterval = 0

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/start", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Carol))
	assert.Equal("40 Already playing another game\r\n", server.Handle("/users/checkers/join/2", server.Carol))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))
	assert.Equal("30 /users/checkers/2\r\n", server.Handle("/users/checkers/join/2", server.Carol))
}

func TestCheckers_MustCapture(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("40 Must capture\r\n", server.Handle("/users/checkers/move/1/647300", server.Alice))
}

func TestCheckers_NotYourTurn(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("40 Wait for your turn\r\n", server.Handle("/users/checkers/move/1/537564", server.Bob))
}

func TestCheckers_InvalidMove(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("40 Invalid move\r\n", server.Handle("/users/checkers/move/1/643153", server.Alice))
}

func TestCheckers_SurrenderHuman(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "bob won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "You won.")
}

func TestCheckers_SurrenderHumanMove(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "bob won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "You won.")

	assert.Equal("40 Error\r\n", server.Handle("/users/checkers/move/1/644253", server.Alice))
}

func TestCheckers_SurrenderOrc(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/150400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/021300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/756400", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/425300", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Bob))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "alice won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "You won.")
}
