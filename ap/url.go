/*
Copyright 2025 Dima Krasner

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

package ap

import (
	"errors"
	"net/url"
	"regexp"
)

var portableIDFormat = regexp.MustCompile(`^(ap://(did:key:z[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+))[/#?].*`)

func CanonicalizeActorID(id string) (string, error) {
	if m := portableIDFormat.FindStringSubmatch(id); m != nil {
		u, err := url.Parse(id[5:])
		if err != nil {
			return "", err
		}

		u.RawQuery = ""
		return "ap://" + u.String(), nil
	}

	if id == "" {
		return "", errors.New("empty actor ID")
	}

	if _, err := url.Parse(id); err != nil {
		return "", err
	}

	return id, nil
}

// GetOrigin returns the origin of an object, based on its ID.
func GetOrigin(id string) (string, error) {
	if m := portableIDFormat.FindStringSubmatch(id); m != nil {
		return m[2], nil
	}

	if u, err := url.Parse(id); err != nil {
		return "", err
	} else {
		return u.Host, nil
	}
}

// SameActor determines whether two IDs represent the same actor.
func SameActor(a, b string) bool {
	if m := portableIDFormat.FindStringSubmatch(a); m != nil {
		a = m[1]
	}

	if m := portableIDFormat.FindStringSubmatch(b); m != nil {
		b = m[1]
	}

	return a == b
}
