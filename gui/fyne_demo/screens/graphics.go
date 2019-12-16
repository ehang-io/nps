package screens

import (
	"image/color"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/theme"
)

func rgbGradient(x, y, w, h int) color.Color {
	g := int(float32(x) / float32(w) * float32(255))
	b := int(float32(y) / float32(h) * float32(255))

	return color.RGBA{uint8(255 - b), uint8(g), uint8(b), 0xff}
}

// GraphicsScreen loads a graphics example panel for the demo app
func GraphicsScreen() fyne.CanvasObject {
	gradient := canvas.NewHorizontalGradient(color.RGBA{0x80, 0, 0, 0xff}, color.RGBA{0, 0x80, 0, 0xff})
	go func() {
		for {
			time.Sleep(time.Second)

			gradient.Angle += 45
			if gradient.Angle >= 360 {
				gradient.Angle -= 360
			}
			canvas.Refresh(gradient)
		}
	}()
	content := fyne.NewContainerWithLayout(layout.NewFixedGridLayout(fyne.NewSize(90, 90)),
		&canvas.Rectangle{FillColor: color.RGBA{0x80, 0, 0, 0xff},
			StrokeColor: color.RGBA{0xff, 0xff, 0xff, 0xff},
			StrokeWidth: 1},
		canvas.NewImageFromResource(theme.FyneLogo()),
		&canvas.Line{StrokeColor: color.RGBA{0, 0, 0x80, 0xff}, StrokeWidth: 5},
		&canvas.Circle{StrokeColor: color.RGBA{0, 0, 0x80, 0xff},
			FillColor:   color.RGBA{0x30, 0x30, 0x30, 0x60},
			StrokeWidth: 2},
		canvas.NewText("Text", color.RGBA{0, 0x80, 0, 0xff}),
		canvas.NewRasterWithPixels(rgbGradient),
		gradient,
		canvas.NewRadialGradient(color.RGBA{0x80, 0, 0, 0xff}, color.RGBA{0, 0x80, 0x80, 0xff}),
	)

	return fyne.NewContainerWithLayout(layout.NewAdaptiveGridLayout(2), content, IconsPanel())
}
