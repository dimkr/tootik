/*
Copyright 2023 - 2025 Dima Krasner

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
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/data"
	"log/slog"
)

type ActivityType string

const (
	Create   ActivityType = "Create"
	Follow   ActivityType = "Follow"
	Accept   ActivityType = "Accept"
	Undo     ActivityType = "Undo"
	Delete   ActivityType = "Delete"
	Announce ActivityType = "Announce"
	Update   ActivityType = "Update"
	Like     ActivityType = "Like"
	Dislike  ActivityType = "Dislike"
	Move     ActivityType = "Move"
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

// Activity represents an ActivityPub activity.
// Object can point to another Activity, an [Object] or a string.
type Activity struct {
	Context   any          `json:"@context,omitempty"`
	ID        string       `json:"id"`
	Type      ActivityType `json:"type"`
	Actor     string       `json:"actor"`
	Object    any          `json:"object"`
	Target    string       `json:"target,omitempty"`
	To        Audience     `json:"to,omitempty"`
	CC        Audience     `json:"cc,omitempty"`
	Published *Time        `json:"published,omitempty"`
}

// RawActivity is a serialized or serializable [Activity]
type RawActivity interface {
	data.JSON | *Activity
}

var (
	ErrInvalidActivity = errors.New("invalid activity")

	knownActivityTypes = map[ActivityType]struct{}{
		Create:   struct{}{},
		Follow:   struct{}{},
		Accept:   struct{}{},
		Undo:     struct{}{},
		Delete:   struct{}{},
		Announce: struct{}{},
		Update:   struct{}{},
		Like:     struct{}{},
		Dislike:  struct{}{},
		Move:     struct{}{},
	}
)

func (a *Activity) IsPublic() bool {
	return a.To.Contains(Public) || a.CC.Contains(Public)
}

func (a *Activity) UnmarshalJSON(b []byte) error {
	var common anyActivity
	if err := json.Unmarshal(b, &common); err != nil {
		return err
	}

	if _, ok := knownActivityTypes[common.Type]; !ok {
		return ErrInvalidActivity
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

func (a *Activity) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("unsupported conversion from %T to %T", src, a)
	}
	return json.Unmarshal([]byte(s), a)
}

func (a *Activity) Value() (driver.Value, error) {
	buf, err := json.Marshal(a)
	return string(buf), err
}

func (a *Activity) LogValue() slog.Value {
	if o, ok := a.Object.(*Object); ok {
		return slog.GroupValue(slog.String("id", a.ID), slog.String("type", string(a.Type)), slog.String("actor", a.Actor), slog.Group("object", "kind", "object", "id", o.ID, "type", o.Type, "attributed_to", o.AttributedTo))
	} else if inner, ok := a.Object.(*Activity); ok {
		return slog.GroupValue(slog.String("id", a.ID), slog.String("type", string(a.Type)), slog.String("actor", a.Actor), slog.Group("object", "kind", "activity", "id", inner.ID, "type", inner.Type, "actor", inner.Actor))
	} else if s, ok := a.Object.(string); ok {
		return slog.GroupValue(slog.String("id", a.ID), slog.String("type", string(a.Type)), slog.String("actor", a.Actor), slog.Group("object", "kind", "string", "id", s))
	} else {
		return slog.GroupValue(slog.String("id", a.ID), slog.String("type", string(a.Type)), slog.String("actor", a.Actor))
	}
}
