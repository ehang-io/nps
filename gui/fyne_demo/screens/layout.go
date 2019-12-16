package screens

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

func makeCell() fyne.CanvasObject {
	rect := canvas.NewRectangle(&color.RGBA{128, 128, 128, 255})
	rect.SetMinSize(fyne.NewSize(30, 30))
	return rect
}

func makeBorderLayout() *fyne.Container {
	top := makeCell()
	bottom := makeCell()
	left := makeCell()
	right := makeCell()
	middle := widget.NewLabelWithStyle("BorderLayout", fyne.TextAlignCenter, fyne.TextStyle{})

	borderLayout := layout.NewBorderLayout(top, bottom, left, right)
	return fyne.NewContainerWithLayout(borderLayout,
		top, bottom, left, right, middle)
}

func makeBoxLayout() *fyne.Container {
	top := makeCell()
	bottom := makeCell()
	middle := widget.NewLabel("BoxLayout")
	center := makeCell()
	right := makeCell()

	col := fyne.NewContainerWithLayout(layout.NewVBoxLayout(),
		top, middle, bottom)

	return fyne.NewContainerWithLayout(layout.NewHBoxLayout(),
		col, center, right)
}

func makeFixedGridLayout() *fyne.Container {
	box1 := makeCell()
	box2 := widget.NewLabel("FixedGrid")
	box3 := makeCell()
	box4 := makeCell()

	return fyne.NewContainerWithLayout(layout.NewFixedGridLayout(fyne.NewSize(75, 75)),
		box1, box2, box3, box4)
}

func makeGridLayout() *fyne.Container {
	box1 := makeCell()
	box2 := widget.NewLabel("Grid")
	box3 := makeCell()
	box4 := makeCell()

	return fyne.NewContainerWithLayout(layout.NewGridLayout(2),
		box1, box2, box3, box4)
}

// LayoutPanel loads a panel that shows the layouts available for a container
func LayoutPanel() fyne.CanvasObject {
	return widget.NewTabContainer(
		widget.NewTabItem("Border", makeBorderLayout()),
		widget.NewTabItem("Box", makeBoxLayout()),
		widget.NewTabItem("Fixed Grid", makeFixedGridLayout()),
		widget.NewTabItem("Grid", makeGridLayout()),
	)
}
