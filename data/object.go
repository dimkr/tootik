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

package data

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Object struct {
	ID       string
	Hash     string
	Type     string
	Actor    string
	Object   string
	Inserted time.Time
}

type ObjectsTable struct{}

var Objects = ObjectsTable{}

func (ObjectsTable) Create(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS objects(id STRING NOT NULL PRIMARY KEY, hash STRING NOT NULL, type STRING NOT NULL, actor STRING NOT NULL, object JSON NOT NULL, inserted INTEGER);`)
	return err
}

func (ObjectsTable) GetByID(id string, db *sql.DB) (*Object, error) {
	row := db.QueryRow(`SELECT * FROM objects WHERE id = ?`, id)

	o := Object{}
	var s string
	var i int64
	if err := row.Scan(&o.ID, &o.Hash, &o.Type, &o.Actor, &s, &i); err != nil {
		return nil, err
	}
	o.Inserted = time.Unix(i, 0)
	if err := json.Unmarshal([]byte(s), &o.Object); err != nil {
		o.Object = s
		return &o, nil
	}
	return &o, nil
}

func (ObjectsTable) GetByHash(hash string, db *sql.DB) (*Object, error) {
	row := db.QueryRow(`SELECT * FROM objects WHERE hash = ?`, hash)

	o := Object{}
	var s string
	var i int64
	if err := row.Scan(&o.ID, &o.Hash, &o.Type, &o.Actor, &s, &i); err != nil {
		return nil, err
	}
	o.Inserted = time.Unix(i, 0)
	if err := json.Unmarshal([]byte(s), &o.Object); err != nil {
		o.Object = s
		return &o, nil
	}
	return &o, nil
}

func (ObjectsTable) Insert(db *sql.DB, o *Object) error {
	_, err := db.Exec(`INSERT INTO objects VALUES(?,?,?,?,?,?);`, o.ID, fmt.Sprintf("%x", sha256.Sum256([]byte(o.ID))), o.Type, o.Actor, o.Object, time.Now().Unix())
	return err
}
