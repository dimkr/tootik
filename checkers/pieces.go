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

package checkers

import "encoding/json"

type pieces map[Coord]Piece

func (p pieces) MarshalJSON() ([]byte, error) {
	tmp := make([]struct {
		Coord
		Piece
	}, len(p))

	i := 0
	for pos, piece := range p {
		tmp[i].Coord = pos
		tmp[i].Piece = piece
		i++
	}

	return json.Marshal(tmp)
}

func (p *pieces) UnmarshalJSON(b []byte) error {
	var tmp []struct {
		Coord
		Piece
	}

	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	*p = make(pieces, len(tmp))

	for _, item := range tmp {
		(*p)[item.Coord] = item.Piece
	}

	return nil
}
