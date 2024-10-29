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
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestCheckers_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/6442", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5133", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3344", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3553", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/6244", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0615", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/2233", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0422", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3113", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4655", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1324", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/2635", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/2406", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3553", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3324", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5362", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7153", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5375", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5746", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7557", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5735", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7766", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/2415", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1526", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3715", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0624", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/6655", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/2415", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5544", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/3553", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5375", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1706", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7557", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0624", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/5702", server.Bob))

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

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("40 Must capture\r\n", server.Handle("/users/checkers/move/1/6473", server.Alice))
}

func TestCheckers_NotYourTurn(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("40 Wait for your turn\r\n", server.Handle("/users/checkers/move/1/5375", server.Bob))
}

func TestCheckers_InvalidMove(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("40 Invalid move\r\n", server.Handle("/users/checkers/move/1/643153", server.Alice))
}

func TestCheckers_SurrenderHuman(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "bob won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "You won.")
}

func TestCheckers_SurrenderHumanMove(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Alice))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "bob won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "You won.")

	assert.Equal("40 Error\r\n", server.Handle("/users/checkers/move/1/644253", server.Alice))
}

func TestCheckers_SurrenderOrc(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.CheckersRandomizePlayer = nil

	assert := assert.New(t)

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/start", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/join/1", server.Bob))

	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/1504", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/0213", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/7564", server.Alice))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/move/1/4253", server.Bob))
	assert.Equal("30 /users/checkers/1\r\n", server.Handle("/users/checkers/surrender/1", server.Bob))

	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Bob), "\n"), "alice won.")
	assert.Contains(strings.Split(server.Handle("/users/checkers/1", server.Alice), "\n"), "You won.")
}
