package screens

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/theme"
)

var (
	purple = &color.RGBA{R: 128, G: 0, B: 128, A: 255}
	orange = &color.RGBA{R: 198, G: 123, B: 0, A: 255}
	grey   = &color.Gray{Y: 123}
)

// customTheme is a simple demonstration of a bespoke theme loaded by a Fyne app.
type customTheme struct {
}

func (customTheme) BackgroundColor() color.Color {
	return purple
}

func (customTheme) ButtonColor() color.Color {
	return color.Black
}

func (customTheme) DisabledButtonColor() color.Color {
	return color.White
}

func (customTheme) HyperlinkColor() color.Color {
	return orange
}

func (customTheme) TextColor() color.Color {
	return color.White
}

func (customTheme) DisabledTextColor() color.Color {
	return color.Black
}

func (customTheme) IconColor() color.Color {
	return color.White
}

func (customTheme) DisabledIconColor() color.Color {
	return color.Black
}

func (customTheme) PlaceHolderColor() color.Color {
	return grey
}

func (customTheme) PrimaryColor() color.Color {
	return orange
}

func (customTheme) HoverColor() color.Color {
	return orange
}

func (customTheme) FocusColor() color.Color {
	return orange
}

func (customTheme) ScrollBarColor() color.Color {
	return grey
}

func (customTheme) ShadowColor() color.Color {
	return &color.RGBA{0xcc, 0xcc, 0xcc, 0xcc}
}

func (customTheme) TextSize() int {
	return 12
}

func (customTheme) TextFont() fyne.Resource {
	return theme.DefaultTextBoldFont()
}

func (customTheme) TextBoldFont() fyne.Resource {
	return theme.DefaultTextBoldFont()
}

func (customTheme) TextItalicFont() fyne.Resource {
	return theme.DefaultTextBoldItalicFont()
}

func (customTheme) TextBoldItalicFont() fyne.Resource {
	return theme.DefaultTextBoldItalicFont()
}

func (customTheme) TextMonospaceFont() fyne.Resource {
	return theme.DefaultTextMonospaceFont()
}

func (customTheme) Padding() int {
	return 10
}

func (customTheme) IconInlineSize() int {
	return 20
}

func (customTheme) ScrollBarSize() int {
	return 10
}

func (customTheme) ScrollBarSmallSize() int {
	return 5
}

func newCustomTheme() fyne.Theme {
	return &customTheme{}
}
