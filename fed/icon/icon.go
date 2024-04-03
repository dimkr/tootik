/*
Copyright 2023, 2024 Dima Krasner

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

// Package icon generates tiny, pseudo-random user avatars.
package icon

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
)

const (
	MediaType         = "image/gif"
	FileNameExtension = ".gif"
)

// Generate generates a tiny pseudo-random image by user name
func Generate(s string) ([]byte, error) {
	hash := sha256.Sum256([]byte(s))

	fg := color.RGBA{128 + (hash[0]^hash[29])%128, 128 + (hash[1]^hash[30])%128, 128 + (hash[2]^hash[31])%128, 255}
	alt := []color.RGBA{
		color.RGBA{fg.R, fg.B, fg.G, 255},
		color.RGBA{fg.G, fg.B, fg.R, 255},
		color.RGBA{fg.G, fg.R, fg.B, 255},
		color.RGBA{fg.B, fg.R, fg.G, 255},
		color.RGBA{fg.B, fg.G, fg.R, 255},
	}[hash[0]%5]
	bg := color.RGBA{255 - fg.R, 255 - fg.G, 255 - fg.B, 255}

	m := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{bg, fg, alt})
	draw.Draw(m, m.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	for i, b := range hash[:16] {
		c := fg
		if hash[16+i]%8 == 0 {
			c = alt
		}

		if (b^hash[16+i])%2 == 1 {
			m.Set(i%4, i/4, c)
			m.Set(i%4, 7-i/4, c)
			m.Set(7-i%4, 7-i/4, c)
			m.Set(7-i%4, i/4, c)
		}
	}

	var buf bytes.Buffer
	if err := gif.Encode(&buf, m, &gif.Options{NumColors: 3}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
