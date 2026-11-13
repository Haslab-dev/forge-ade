package ui

import "github.com/egoist/mygo/ui"

// LeftResizeHandle renders an interactive draggable divider for a right-anchored panel (e.g. Right Action Drawer).
// Dragging left (negative dx) expands width; dragging right (positive dx) contracts width.
func LeftResizeHandle(c *ui.Context, width *float32, minW, maxW, resetW float32) {
	t := c.Theme()
	handle := ui.Box(c).Width(4).FillHeight().Cursor(ui.CursorResizeEW)
	dx, _, held := handle.Dragged()
	hovered := handle.Hovered()

	if handle.DoubleClicked() {
		*width = resetW
	} else if held && dx != 0 {
		next := *width - dx
		if next < minW {
			next = minW
		}
		if next > maxW {
			next = maxW
		}
		*width = next
	}

	if held || hovered {
		handle.Background(t.Accent)
	} else {
		handle.Background(t.Border)
	}
}

// TopResizeHandle renders an interactive draggable divider for a bottom-anchored panel (e.g. Bottom Terminal).
// Dragging up (negative dy) expands height; dragging down (positive dy) contracts height.
func TopResizeHandle(c *ui.Context, height *float32, minH, maxH, resetH float32) {
	t := c.Theme()
	handle := ui.Box(c).Height(4).FillWidth().Cursor(ui.CursorResizeNS)
	_, dy, held := handle.Dragged()
	hovered := handle.Hovered()

	if handle.DoubleClicked() {
		*height = resetH
	} else if held && dy != 0 {
		next := *height - dy
		if next < minH {
			next = minH
		}
		if next > maxH {
			next = maxH
		}
		*height = next
	}

	if held || hovered {
		handle.Background(t.Accent)
	} else {
		handle.Background(t.Border)
	}
}
