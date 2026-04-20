//go:build windows

// Package appicon embeds the application icon so it can be used at runtime
// (e.g. as the main window icon). The Windows executable icon itself is
// embedded separately via rsrc during the release build.
package appicon

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
)

//go:embed app.png
var appPNG []byte

// PNG returns the raw embedded PNG bytes.
func PNG() []byte {
	return appPNG
}

// Image decodes the embedded PNG into an image.Image.
func Image() (image.Image, error) {
	return png.Decode(bytes.NewReader(appPNG))
}
