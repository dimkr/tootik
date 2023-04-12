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

package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/icon"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"os/exec"
)

// limit the number of child openssl processes
var sem = semaphore.NewWeighted(2)

func gen(ctx context.Context) ([]byte, []byte, error) {
	if err := sem.Acquire(ctx, 1); err != nil {
		return nil, nil, fmt.Errorf("Failed to acquire semaphore: %w", err)
	}
	defer sem.Release(1)

	cmd := exec.CommandContext(ctx, "openssl", "genrsa", "2048")
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("Failed to generate private key: %w", err)
	}

	priv, err := ioutil.ReadAll(&stdout)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to read private key: %w", err)
	}

	cmd = exec.CommandContext(ctx, "openssl", "rsa", "-pubout")
	cmd.Stdin = bytes.NewBuffer(priv)
	stdout = bytes.Buffer{}
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("Failed to generate public key: %w", err)
	}

	pub, err := ioutil.ReadAll(&stdout)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to read public key: %w", err)
	}

	return priv, pub, nil
}

func Create(ctx context.Context, db *sql.DB, id, name, certHash string) (*ap.Actor, error) {
	priv, pub, err := gen(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to generate key pair: %w", err)
	}

	body, err := json.Marshal(
		map[string]any{
			"@context": []string{
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1",
			},
			"id":                id,
			"type":              ap.Person,
			"preferredUsername": name,
			"summary":           fmt.Sprintf(`<p><a href="gemini://%s/users/resolve?%s" target="_blank">♊ Me on Gemini</a></p>`, cfg.Domain, name),
			"icon": map[string]any{
				"type":      ap.ImageAttachment,
				"mediaType": icon.MediaType,
				"url":       fmt.Sprintf("https://%s/icon/%s%s", cfg.Domain, name, icon.FileNameExtension),
			},
			"inbox":     fmt.Sprintf("https://%s/inbox/%s", cfg.Domain, name),
			"outbox":    fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, name),
			"followers": fmt.Sprintf("https://%s/followers/%s", cfg.Domain, name),
			"publicKey": map[string]any{
				"id":           fmt.Sprintf("https://%s/user/%s#main-key", cfg.Domain, name),
				"owner":        id,
				"publicKeyPem": string(pub),
			},
			"manuallyApprovesFollowers": false,

			"privateKey":        string(priv),
			"clientCertificate": certHash,
		})
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal %s: %w", id, err)
	}

	actor := ap.Actor{}
	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal %s: %w", id, err)
	}

	if _, err = db.ExecContext(
		ctx,
		`INSERT INTO persons (id, hash, actor) VALUES(?,?,?)`,
		id,
		fmt.Sprintf("%x", sha256.Sum256([]byte(id))),
		string(body),
	); err != nil {
		return nil, fmt.Errorf("Failed to insert %s: %w", id, err)
	}

	return &actor, nil
}
