package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	iconActive   []byte
	iconInactive []byte
	iconError    []byte
)

func init() {
	iconActive = circle(color.RGBA{0x2e, 0xcc, 0x71, 0xff})
	iconInactive = circle(color.RGBA{0x95, 0xa5, 0xa6, 0xff})
	iconError = circle(color.RGBA{0xe7, 0x4c, 0x3c, 0xff})
}

func circle(c color.RGBA) []byte {
	const n = 22
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	cx, cy, r := float64(n)/2, float64(n)/2, float64(n)/2-1
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := dx*dx + dy*dy
			switch {
			case d <= r*r:
				img.Set(x, y, c)
			case d <= (r+0.5)*(r+0.5):
				img.Set(x, y, color.RGBA{c.R, c.G, c.B, 0x80})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
