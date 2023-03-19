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

package fed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/data"
	"github.com/go-ap/activitypub"
	"github.com/igor-pavlenko/httpsignatures-go"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func send(db *sql.DB, sender *data.Object, actor *activitypub.Actor, req *http.Request) (*http.Response, error) {
	urlString := req.URL.String()

	if sender == nil {
		log.WithFields(log.Fields{"url": urlString, "receiver": actor.ID}).Info("Sending request")
	} else {
		log.WithFields(log.Fields{"url": urlString, "sender": sender.ID, "receiver": actor.ID}).Info("Sending request")
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse %s: %w", urlString, err)
	}

	req.Header.Add("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	if sender != nil {
		key := struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  struct {
				ID           string `json:"id"`
				Owner        string `json:"owner"`
				PublicKeyPem string `json:"publicKeyPem"`
			} `json:"publicKey"`
		}{}
		if err := json.Unmarshal([]byte(sender.Object), &key); err != nil {
			return nil, fmt.Errorf("Sender %s has no key: %w", actor.ID, err)
		}

		secrets := map[string]httpsignatures.Secret{
			key.PublicKey.ID: {
				KeyID:      key.PublicKey.ID,
				PublicKey:  key.PublicKey.PublicKeyPem,
				PrivateKey: key.PrivateKey,
				Algorithm:  "rsa-sha256",
			},
		}
		ss := httpsignatures.NewSimpleSecretsStorage(secrets)
		hs := httpsignatures.NewHTTPSignatures(ss)
		// TODO: drop digest from GET?
		hs.SetDefaultSignatureHeaders([]string{"(request-target)", "host", "date", "digest"})

		var body []byte
		var hash [sha256.Size]byte

		if req.Body == nil {
			hash = sha256.Sum256(nil)
		} else {
			body, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("Failed to read body for %s: %w", urlString, err)
			}

			req.Body = io.NopCloser(bytes.NewReader(body))
			hash = sha256.Sum256(body)
		}

		req.Header.Add("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]))
		req.Header.Add("Date", time.Now().UTC().Format(http.TimeFormat))
		req.Header.Add("Host", u.Host)

		if err := hs.Sign(key.PublicKey.ID, req); err != nil {
			return nil, fmt.Errorf("Failed to sign request for %s: %w", urlString, err)
		}
	}

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request to %s: %w", urlString, err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("Failed to send request to %s: %d, %w", urlString, resp.StatusCode, err)
		}
		return nil, fmt.Errorf("Failed to send request to %s: %d, %s", urlString, resp.StatusCode, string(body))
	}

	return resp, nil
}

func Send(ctx context.Context, db *sql.DB, sender *data.Object, receiver, body string) error {
	actor, err := Resolve(ctx, db, sender, receiver)
	if err != nil {
		return fmt.Errorf("Cannot send message to %s: failed to resolve", receiver)
	}

	if actor.Inbox == nil || !actor.Inbox.IsLink() {
		return fmt.Errorf("Cannot send message to %s: no inbox link", receiver)
	}

	inbox := string(actor.Inbox.GetLink())
	if inbox == "" {
		return fmt.Errorf("Cannot send message to %s: no inbox link", receiver)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader([]byte(body)))
	if err != nil {
		return fmt.Errorf("Failed to send message to %s: %w", receiver, err)
	}
	//req.Header.Add("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Add("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	resp, err := send(db, sender, actor, req)
	if err != nil {
		return fmt.Errorf("Failed to send message to %s: %w", receiver, err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to send message to %s: %w", receiver, err)
	}

	log.WithFields(log.Fields{"actor": receiver, "body": string(respBody)}).Info("Successfully sent message")

	return nil
}
