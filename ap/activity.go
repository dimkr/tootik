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

import (
	"encoding/json"
	"errors"
)

type ActivityType string

const (
	CreateActivity   ActivityType = "Create"
	FollowActivity   ActivityType = "Follow"
	AcceptActivity   ActivityType = "Accept"
	UndoActivity     ActivityType = "Undo"
	DeleteActivity   ActivityType = "Delete"
	AnnounceActivity ActivityType = "Announce"
	UpdateActivity   ActivityType = "Update"
	LikeActivity     ActivityType = "Like"
)

type anyActivity struct {
	Context any             `json:"@context"`
	ID      string          `json:"id"`
	Type    ActivityType    `json:"type"`
	Actor   string          `json:"actor"`
	Object  json.RawMessage `json:"object"`
	To      Audience        `json:"to"`
	CC      Audience        `json:"cc"`
}

type Activity struct {
	Context any          `json:"@context,omitempty"`
	ID      string       `json:"id"`
	Type    ActivityType `json:"type"`
	Actor   string       `json:"actor"`
	Object  any          `json:"object"`
	To      Audience     `json:"to,omitempty"`
	CC      Audience     `json:"cc,omitempty"`
}

var ErrInvalidActivity = errors.New("Invalid activity")

func (a *Activity) UnmarshalJSON(b []byte) error {
	var common anyActivity
	if err := json.Unmarshal(b, &common); err != nil {
		return err
	}

	a.Context = common.Context
	a.ID = common.ID
	a.Type = common.Type
	a.Actor = common.Actor
	a.To = common.To
	a.CC = common.CC

	var object Object
	var activity Activity
	var link string
	if err := json.Unmarshal(common.Object, &activity); err == nil {
		a.Object = &activity
	} else if err := json.Unmarshal(common.Object, &object); err == nil {
		a.Object = &object
	} else if err := json.Unmarshal(common.Object, &link); err == nil {
		a.Object = link
	} else {
		return ErrInvalidActivity
	}

	return nil
}
