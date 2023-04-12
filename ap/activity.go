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

package ap

import "encoding/json"

type ActivityType string

const (
	CreateActivity ActivityType = "Create"
	FollowActivity ActivityType = "Follow"
	AcceptActivity ActivityType = "Accept"
	UndoActivity   ActivityType = "Undo"
	DeleteActivity ActivityType = "Delete"
)

type anyActivity struct {
	ID     string          `json:"id"`
	Type   ActivityType    `json:"type"`
	Actor  string          `json:"actor"`
	Object json.RawMessage `json:"object"`
}

type Activity struct {
	ID     string       `json:"id"`
	Type   ActivityType `json:"type"`
	Actor  string       `json:"actor"`
	Object any          `json:"object"`
}

func (a *Activity) UnmarshalJSON(b []byte) error {
	var common anyActivity
	if err := json.Unmarshal(b, &common); err != nil {
		return err
	}

	a.ID = common.ID
	a.Type = common.Type
	a.Actor = common.Actor

	var object Object
	var link string
	if err := json.Unmarshal(common.Object, &object); err != nil {
		if err := json.Unmarshal(common.Object, &link); err != nil {
			return err
		}
		a.Object = link
	} else {
		a.Object = &object
	}

	return nil
}
