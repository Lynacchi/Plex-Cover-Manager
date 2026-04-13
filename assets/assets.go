package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed app.png
var appIconPNG []byte

func AppIcon() fyne.Resource {
	return fyne.NewStaticResource("plex-cover-manager.png", appIconPNG)
}
