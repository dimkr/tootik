/*
Copyright 2026 Dima Krasner

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

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// runMainEnv makes the test binary run main with its own arguments instead of running tests.
const runMainEnv = "TOOTIK_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func TestMain_RequiresLake(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-domain", "localhost.localdomain")
	cmd.Env = append(os.Environ(), runMainEnv+"=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Expected tootik to exit with an error: %v", err)
	}

	assert.Equal(t, 2, exitErr.ExitCode())
	assert.Contains(t, stderr.String(), "-lake")
}
