package data

import (
	"fyne.io/fyne"
)

// ThemedResource is a resource wrapper that will return an appropriate resource
// for the currently selected theme.
type ThemedResource struct {
	dark, light fyne.Resource
}

func isLight() bool {
	r, g, b, _ := fyne.CurrentApp().Settings().Theme().TextColor().RGBA()
	return r < 0xaaaa && g < 0xaaaa && b < 0xaaaa
}

// Name returns the underlying resource name (used for caching)
func (res *ThemedResource) Name() string {
	if isLight() {
		return res.light.Name()
	}
	return res.dark.Name()
}

// Content returns the underlying content of the correct resource for the current theme
func (res *ThemedResource) Content() []byte {
	if isLight() {
		return res.light.Content()
	}
	return res.dark.Content()
}

// NewThemedResource creates a resource that adapts to the current theme setting.
func NewThemedResource(dark, light fyne.Resource) *ThemedResource {
	return &ThemedResource{dark, light}
}

// FyneScene contains the full fyne logo with background design
var FyneScene = NewThemedResource(fynescenedark, fynescenelight)
