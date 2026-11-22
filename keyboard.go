package main

import "strconv"

// Keyboard navigation for the panel. Tab or Down moves the focus to the next control, Shift+Tab or Up to the previous one.
// Left and Right move a slider by 5. Enter and Space press the focused button, check box or link.
// The focus ring is drawn only after a key press; a mouse click hides it again.

var (
	focusID    string
	kbdFocus   bool
	pKeyState  = user32.NewProc("GetKeyState")
	pRegHotKey = user32.NewProc("RegisterHotKey")
	presetDisp = -1 // -1 = all displays, otherwise the index of the display the presets and hotkeys change
)

// focusable lists the items that can take the keyboard focus, in layout order.
func focusable(items []item) []item {
	var out []item
	for _, it := range items {
		if it.kind == kSlider || it.kind == kButton || it.kind == kCheck || it.kind == kLink {
			out = append(out, it)
		}
	}
	return out
}

func focusedItem(items []item) *item {
	for _, it := range focusable(items) {
		if it.id == focusID {
			it := it
			return &it
		}
	}
	return nil
}

func moveFocus(items []item, step int) {
	fs := focusable(items)
	if len(fs) == 0 {
		return
	}
	i := -1
	for k, it := range fs {
		if it.id == focusID {
			i = k
			break
		}
	}
	if i < 0 && step < 0 {
		i = len(fs)
	}
	focusID = fs[((i+step)%len(fs)+len(fs))%len(fs)].id
	kbdFocus = true
}

// keyDown handles one key press in the panel. Esc is handled by the window procedure.
func keyDown(vk uintptr) {
	s := snap()
	items, _ := build(s)
	shift := call(pKeyState, 0x10)&0x8000 != 0
	switch vk {
	case 0x09: // Tab
		if shift {
			moveFocus(items, -1)
		} else {
			moveFocus(items, 1)
		}
	case 0x26: // Up
		moveFocus(items, -1)
	case 0x28: // Down
		moveFocus(items, 1)
	case 0x25, 0x27: // Left, Right
		if f := focusedItem(items); f != nil && f.kind == kSlider {
			d := 5
			if vk == 0x25 {
				d = -5
			}
			setSliderValue(*f, min(max(sliderValue(s, f.id)+d, 0), 100))
		}
	case 0x0D, 0x20: // Enter, Space
		if f := focusedItem(items); f != nil && f.kind != kSlider {
			activate(*f)
		}
	default:
		return
	}
	call(pInvalidate, hwnd, 0, 0)
}

// drawRing draws a 2 px outline around the item that has the keyboard focus.
func drawRing(p *painter, it item, col uint32) {
	if !kbdFocus || it.id != focusID || (it.kind != kSlider && it.kind != kButton && it.kind != kCheck && it.kind != kLink) {
		return
	}
	x, y, w, h := int(it.r.l), int(it.r.t), int(it.r.r-it.r.l), int(it.r.b-it.r.t)
	t := px(2)
	p.box(x-t, y-t, w+2*t, t, col)
	p.box(x-t, y+h, w+2*t, t, col)
	p.box(x-t, y, t, h, col)
	p.box(x+w, y, t, h, col)
}

// Global hotkeys (RegisterHotKey, no keyboard hook). Ctrl+Alt+Up and Ctrl+Alt+Down change the brightness by 10,
// Ctrl+Alt+M mutes or unmutes the speakers. A combination another program already uses is skipped silently.
func registerHotkeys() {
	const mods = 0x0001 | 0x0002 | 0x4000 // Alt | Ctrl | no auto-repeat
	call(pRegHotKey, hwnd, 1, mods, 0x26) // Up
	call(pRegHotKey, hwnd, 2, mods, 0x28) // Down
	call(pRegHotKey, hwnd, 3, mods, 0x4D) // M
}

func hotkey(id uintptr) {
	switch id {
	case 1:
		stepBrightness(10)
	case 2:
		stepBrightness(-10)
	case 3:
		activate(item{kind: kButton, id: "mute"})
	}
}

// stepBrightness changes the brightness of the selected display, or of every display when none is selected.
func stepBrightness(d int) {
	type change struct{ i, v int }
	var changes []change
	mu.Lock()
	for i := range disp {
		if presetDisp < 0 || presetDisp == i {
			disp[i].val = min(max(disp[i].val+d, 0), 100)
			changes = append(changes, change{i, disp[i].val})
		}
	}
	night = false
	mu.Unlock()
	for _, c := range changes {
		latest("b"+strconv.Itoa(c.i), c.v, func(x int) { setBrightness(c.i, x) })
	}
	call(pInvalidate, hwnd, 0, 0)
}
