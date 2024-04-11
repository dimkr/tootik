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

package icon

import (
	"bytes"
	"errors"
	"github.com/dimkr/tootik/cfg"
	xdraw "golang.org/x/image/draw"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func Scale(cfg *cfg.Config, data []byte) ([]byte, error) {
	dim, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if dim.Height > cfg.MaxAvatarHeight || dim.Width > cfg.MaxAvatarWidth {
		return nil, errors.New("too big")
	}

	im, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if dim.Height > cfg.AvatarHeight || dim.Width > cfg.AvatarWidth {
		bounds := image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{cfg.AvatarWidth, cfg.AvatarHeight}}
		scaled := image.NewRGBA(bounds)
		xdraw.NearestNeighbor.Scale(scaled, bounds, im, im.Bounds(), draw.Over, nil)
		im = scaled
	}

	var b bytes.Buffer
	if err := gif.Encode(&b, im, &gif.Options{NumColors: 256}); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
