//go:build windows

// Lumistar: a small brightness and volume flyout for the Windows system tray.
// Pure Go, standard library only. The panel is a native window drawn with GDI+.
package main

import (
	_ "embed"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	gdiplus  = syscall.NewLazyDLL("gdiplus.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dxva2    = syscall.NewLazyDLL("dxva2.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClass = user32.NewProc("RegisterClassExW")
	pCreateWindow  = user32.NewProc("CreateWindowExW")
	pDefProc       = user32.NewProc("DefWindowProcW")
	pGetMessage    = user32.NewProc("GetMessageW")
	pTranslate     = user32.NewProc("TranslateMessage")
	pDispatch      = user32.NewProc("DispatchMessageW")
	pCursor        = user32.NewProc("GetCursorPos")
	pMetrics       = user32.NewProc("GetSystemMetrics")
	pCreateMenu    = user32.NewProc("CreatePopupMenu")
	pAppendMenu    = user32.NewProc("AppendMenuW")
	pTrackMenu     = user32.NewProc("TrackPopupMenu")
	pDestroyMenu   = user32.NewProc("DestroyMenu")
	pSetFG         = user32.NewProc("SetForegroundWindow")
	pLoadImage     = user32.NewProc("LoadImageW")
	pLoadCursor    = user32.NewProc("LoadCursorW")
	pEnumMonitors  = user32.NewProc("EnumDisplayMonitors")
	pBeginPaint    = user32.NewProc("BeginPaint")
	pEndPaint      = user32.NewProc("EndPaint")
	pInvalidate    = user32.NewProc("InvalidateRect")
	pShowWindow    = user32.NewProc("ShowWindow")
	pSetWindowPos  = user32.NewProc("SetWindowPos")
	pSetCapture    = user32.NewProc("SetCapture")
	pReleaseCap    = user32.NewProc("ReleaseCapture")
	pDrawText      = user32.NewProc("DrawTextW")
	pFillRect      = user32.NewProc("FillRect")
	pMonFromPt     = user32.NewProc("MonitorFromPoint")
	pMonInfo       = user32.NewProc("GetMonitorInfoW")
	pPostMessage   = user32.NewProc("PostMessageW")
	pRegMessage    = user32.NewProc("RegisterWindowMessageW")
	pSetDPIAware   = user32.NewProc("SetProcessDPIAware")
	pGetDPI        = user32.NewProc("GetDpiForSystem")
	pIsVisible     = user32.NewProc("IsWindowVisible")
	pDwmAttr       = dwmapi.NewProc("DwmSetWindowAttribute")

	pCompatDC   = gdi32.NewProc("CreateCompatibleDC")
	pCompatBmp  = gdi32.NewProc("CreateCompatibleBitmap")
	pSelect     = gdi32.NewProc("SelectObject")
	pDeleteObj  = gdi32.NewProc("DeleteObject")
	pDeleteDC   = gdi32.NewProc("DeleteDC")
	pBitBlt     = gdi32.NewProc("BitBlt")
	pCreateFont = gdi32.NewProc("CreateFontW")
	pSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pBkMode     = gdi32.NewProc("SetBkMode")
	pTextColor  = gdi32.NewProc("SetTextColor")

	pGdipStart    = gdiplus.NewProc("GdiplusStartup")
	pGdipGraphics = gdiplus.NewProc("GdipCreateFromHDC")
	pGdipDelGfx   = gdiplus.NewProc("GdipDeleteGraphics")
	pGdipSmooth   = gdiplus.NewProc("GdipSetSmoothingMode")
	pGdipFill     = gdiplus.NewProc("GdipCreateSolidFill")
	pGdipDelBrush = gdiplus.NewProc("GdipDeleteBrush")
	pGdipRect     = gdiplus.NewProc("GdipFillRectangleI")
	pGdipEllipse  = gdiplus.NewProc("GdipFillEllipseI")
	pGdipPolygon  = gdiplus.NewProc("GdipFillPolygonI")

	pNotify      = shell32.NewProc("Shell_NotifyIconW")
	pModule      = kernel32.NewProc("GetModuleHandleW")
	pMutex       = kernel32.NewProc("CreateMutexW")
	pNumPhys     = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	pGetPhys     = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	pDestroyPhys = dxva2.NewProc("DestroyPhysicalMonitors")
	pGetBright   = dxva2.NewProc("GetMonitorBrightness")
	pSetBright   = dxva2.NewProc("SetMonitorBrightness")
	pCoInit      = ole32.NewProc("CoInitializeEx")
	pCoCreate    = ole32.NewProc("CoCreateInstance")
)

func call(p *syscall.LazyProc, a ...uintptr) uintptr { r, _, _ := p.Call(a...); return r }

// ptr hands a Go pointer to Windows. Storing it in escapeSink forces the object onto the heap, where it can never move.
// (A stack variable could be moved by Go while Windows still writes to its old address, which corrupts memory.)
func ptr[T any](v *T) uintptr {
	escapeSink = unsafe.Pointer(v)
	return uintptr(unsafe.Pointer(v))
}
func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func iu(v int) uintptr     { return uintptr(int32(v)) }

const (
	wmTray = 0x8001
	wmDone = 0x8002
)

type point struct{ x, y int32 }
type rect struct{ l, t, r, b int32 }
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}
type wndClass struct {
	size       uint32
	style      uint32
	proc       uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menu       *uint16
	class      *uint16
	iconSm     uintptr
}
type guid struct {
	d1 uint32
	d2 uint16
	d3 uint16
	d4 [8]byte
}
type notifyIconData struct {
	size     uint32
	hwnd     uintptr
	id       uint32
	flags    uint32
	callback uint32
	icon     uintptr
	tip      [128]uint16
	state    uint32
	stateMsk uint32
	info     [256]uint16
	timeout  uint32
	title    [64]uint16
	infoFlag uint32
	guid     guid
	balloon  uintptr
}
type paintStruct struct {
	hdc      uintptr
	erase    int32
	rc       rect
	restore  int32
	incUpd   int32
	reserved [32]byte
}
type monitorInfo struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

var (
	hwnd                   uintptr
	nid                    notifyIconData
	scale                  = 100
	fonts                  [5]uintptr
	lastHide               time.Time
	taskbarMsg             uint32
	drag                   string
	curX, curY, curW, curH int
	sClass                 = u16("LumistarPanel")
	sOpen                  = u16("Open Lumistar")
	sQuit                  = u16("Quit")
)

func px(v int) int { return v * scale / 100 }

// ---------------------------------------------------------------- state

type dispState struct {
	id       string
	name     string
	val      int
	mon      int
	internal bool
}

var (
	mu         sync.Mutex
	disp       []dispState
	loaded     bool
	vol        int
	muted      bool
	light      bool
	autostart  bool
	night      bool
	awake      bool
	nightSaved []int
	help       bool
	langPage   bool
	langPg     int
	micMuted   bool
)

const nightLevel = 20

type snapshot struct {
	disp                                                                    []dispState
	loaded, muted, light, autostart, night, awake, help, langPage, micMuted bool
	vol, lang, langPg                                                       int
}

func snap() snapshot {
	mu.Lock()
	defer mu.Unlock()
	return snapshot{disp: append([]dispState(nil), disp...), loaded: loaded, muted: muted, light: light,
		autostart: autostart, night: night, awake: awake, help: help, langPage: langPage, langPg: langPg, micMuted: micMuted, vol: vol, lang: lang}
}

// All settings live in %LOCALAPPDATA%\Lumistar, whichever folder Lumistar was installed to.
func settingsPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Lumistar", "settings.txt")
}

func loadTheme() {
	b, _ := os.ReadFile(settingsPath())
	text := string(b)
	light = strings.Contains(text, "theme=light")
	for i, l := range langs {
		if strings.Contains(text, "language="+l.code+"\r") || strings.Contains(text, "language="+l.code+"\n") {
			lang = i
		}
	}
}

// saveTheme writes both settings (theme and language) to settings.txt.
func saveTheme(_ bool) {
	t := "theme=dark\r\n"
	if light {
		t = "theme=light\r\n"
	}
	t += "language=" + langs[lang].code + "\r\n"
	os.MkdirAll(filepath.Dir(settingsPath()), 0o755)
	os.WriteFile(settingsPath(), []byte(t), 0o644)
}

var busy int32

func refresh(full bool) {
	if !atomic.CompareAndSwapInt32(&busy, 0, 1) {
		return
	}
	go func() {
		defer atomic.StoreInt32(&busy, 0)
		defer guard()
		if full {
			enumerateDisplays()
		} else {
			readValues()
		}
		v, m := getVolume()
		mm, _ := getMic()
		a := autostart
		if full {
			a = autostartOn()
		}
		mu.Lock()
		vol, muted, autostart, loaded, micMuted = v, m, a, true, mm
		mu.Unlock()
		call(pPostMessage, hwnd, wmDone, 0, 0)
	}()
}

// Applies only the newest value for each key, so fast slider drags never queue up.
var (
	pendMu  sync.Mutex
	pending = map[string]*int{}
)

func latest(key string, v int, f func(int)) {
	pendMu.Lock()
	if p, ok := pending[key]; ok {
		*p = v
		pendMu.Unlock()
		return
	}
	pending[key] = &v
	pendMu.Unlock()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logPanic(r)
				pendMu.Lock()
				delete(pending, key)
				pendMu.Unlock()
			}
		}()
		for {
			pendMu.Lock()
			cur := *pending[key]
			pendMu.Unlock()
			f(cur)
			pendMu.Lock()
			if *pending[key] == cur {
				delete(pending, key)
				pendMu.Unlock()
				return
			}
			pendMu.Unlock()
		}
	}()
}

// ---------------------------------------------------------------- main + tray

func main() {
	runtime.LockOSThread()
	if _, _, e := pMutex.Call(0, 0, strPtr("Lumistar.SingleInstance")); e == syscall.Errno(183) {
		return
	}
	call(pSetDPIAware)
	if d := int(call(pGetDPI)); d > 0 {
		scale = d * 100 / 96
	}
	loadTheme()
	go audioLoop()

	var token uintptr
	in := [4]uintptr{1, 0, 0, 0}
	call(pGdipStart, ptr(&token), ptr(&in), 0)
	call(pGdipBitmap, 128, 128, 128*4, 0x26200A, uintptr(unsafe.Pointer(&starRaw[0])), ptr(&starImg))
	call(pGdipBitmap, 64, 64, 64*4, 0x26200A, uintptr(unsafe.Pointer(&ghWhiteRaw[0])), ptr(&ghWhiteImg))
	call(pGdipBitmap, 64, 64, 64*4, 0x26200A, uintptr(unsafe.Pointer(&ghDarkRaw[0])), ptr(&ghDarkImg))
	call(pGdipBitmap, 64, 64, 64*4, 0x26200A, uintptr(unsafe.Pointer(&claudeRaw[0])), ptr(&claudeImg))
	loadFlags()

	mkFont := func(h, weight int) uintptr {
		return call(pCreateFont, iu(-px(h)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, strPtr("Segoe UI"))
	}
	fonts = [5]uintptr{mkFont(14, 400), mkFont(15, 600), mkFont(12, 600), mkFont(12, 400), mkFont(11, 600)}

	inst := call(pModule, 0)
	big := call(pLoadImage, inst, 1, 1, uintptr(call(pMetrics, 11)), uintptr(call(pMetrics, 12)), 0)
	small := call(pLoadImage, inst, 1, 1, uintptr(call(pMetrics, 49)), uintptr(call(pMetrics, 50)), 0)
	wc := wndClass{style: 0x20000, instance: inst, class: sClass, icon: big, iconSm: small,
		cursor: call(pLoadCursor, 0, 32512), proc: syscall.NewCallback(wndProc)}
	wc.size = uint32(unsafe.Sizeof(wc))
	call(pRegisterClass, ptr(&wc))
	// Tool window: never appears on the taskbar or in Alt+Tab.
	hwnd = call(pCreateWindow, 0x88, uintptr(unsafe.Pointer(sClass)), strPtr("Lumistar"),
		0x80000000, 0, 0, uintptr(px(320)), 100, 0, 0, inst, 0)
	round := int32(2)
	call(pDwmAttr, hwnd, 33, ptr(&round), 4)
	taskbarMsg = uint32(call(pRegMessage, strPtr("TaskbarCreated")))

	nid.size = uint32(unsafe.Sizeof(nid))
	nid.hwnd, nid.id, nid.flags, nid.callback, nid.icon = hwnd, 1, 0x7, wmTray, small
	copy(nid.tip[:], syscall.StringToUTF16("Lumistar"))
	call(pNotify, 0, ptr(&nid))

	refresh(true)
	var m msg
	for int32(call(pGetMessage, ptr(&m), 0, 0, 0)) > 0 {
		call(pTranslate, ptr(&m))
		call(pDispatch, ptr(&m))
	}
}

func wndProc(h, m, wp, lp uintptr) uintptr {
	defer guard()
	switch m {
	case wmTray:
		switch uint32(lp) {
		case 0x202:
			if call(pIsVisible, hwnd) != 0 {
				hidePanel()
			} else if time.Since(lastHide) > 300*time.Millisecond {
				showPanel()
			}
		case 0x205:
			showMenu()
		}
		return 0
	case 0x111:
		switch wp & 0xFFFF {
		case 1:
			showPanel()
		case 2:
			quit()
		}
		return 0
	case 0x20A: // mouse wheel scrolls the help page when it is taller than the screen
		mu.Lock()
		onHelp := help
		mu.Unlock()
		if onHelp {
			helpScroll = max(helpScroll-int(int16(wp>>16))*px(40)/120, 0)
			call(pInvalidate, hwnd, 0, 0)
		}
		return 0
	case 0x14:
		return 1
	case 0xF:
		paint()
		return 0
	case 0x201:
		mouseDown(lp)
		return 0
	case 0x200:
		if drag != "" {
			mouseMove(lp)
		}
		return 0
	case 0x202:
		drag = ""
		call(pReleaseCap)
		return 0
	case 0x100:
		if wp == 0x1B {
			hidePanel()
		}
		return 0
	case 0x6:
		if wp&0xFFFF == 0 && call(pIsVisible, hwnd) != 0 {
			hidePanel()
		}
	case wmDone:
		if call(pIsVisible, hwnd) != 0 {
			relayout()
		}
		call(pInvalidate, hwnd, 0, 0)
		return 0
	case 0x7E:
		refresh(true)
		return 0
	}
	if taskbarMsg != 0 && uint32(m) == taskbarMsg {
		call(pNotify, 0, ptr(&nid))
		return 0
	}
	return call(pDefProc, h, m, wp, lp)
}

func showMenu() {
	var pt point
	call(pCursor, ptr(&pt))
	h := call(pCreateMenu)
	call(pAppendMenu, h, 0, 1, uintptr(unsafe.Pointer(sOpen)))
	call(pAppendMenu, h, 0, 2, uintptr(unsafe.Pointer(sQuit)))
	call(pSetFG, hwnd)
	call(pTrackMenu, h, 0x22, iu(int(pt.x)), iu(int(pt.y)), 0, hwnd, 0)
	call(pDestroyMenu, h)
}

func hidePanel() {
	call(pShowWindow, hwnd, 0)
	mu.Lock()
	help = false
	langPage, langPg, helpScroll = false, 0, 0
	mu.Unlock()
	lastHide = time.Now()
}

func showPanel() {
	var pt point
	call(pCursor, ptr(&pt))
	var mi monitorInfo
	mi.size = uint32(unsafe.Sizeof(mi))
	hm := call(pMonFromPt, uintptr(uint32(pt.x))|uintptr(uint32(pt.y))<<32, 2)
	call(pMonInfo, hm, ptr(&mi))
	workH = int(mi.work.b-mi.work.t) - px(24)
	st := snap()
	_, h := build(st)
	w := panelW(st.help)
	x := min(max(int(pt.x)-w/2, int(mi.work.l)+px(8)), int(mi.work.r)-w-px(8))
	y := int(mi.work.b) - h - px(12)
	if int(pt.y) < int(mi.work.t) { // taskbar at the top
		y = int(mi.work.t) + px(12)
	}
	curX, curY, curW, curH = x, y, w, h
	call(pSetWindowPos, hwnd, ^uintptr(0), iu(x), iu(y), uintptr(w), iu(h), 0x40)
	call(pSetFG, hwnd)
	refresh(false)
}

func quit() {
	call(pNotify, 2, ptr(&nid))
	os.Exit(0)
}

// ---------------------------------------------------------------- layout

const (
	kCard = iota
	kText
	kSlider
	kButton
	kCheck
	kStar
	kLink
	kBadge
)
const (
	tLeft   = 0x4 | 0x20 | 0x8000 | 0x800
	tRight  = 0x2 | 0x4 | 0x20 | 0x800
	tCenter = 0x1 | 0x4 | 0x20 | 0x800 | 0x8000
	tWrap   = 0x10 | 0x800
)

type item struct {
	kind  int
	r     rect
	text  string
	id    string
	flags uintptr
	font  int
	col   int // 0 text, 1 muted, 2 white
	on    bool
	icon  int
	flag  string
}

func rc(l, t, r, b int) rect { return rect{int32(l), int32(t), int32(r), int32(b)} }

// isIconBtn reports a button that shows only a picture (no text).
func isIconBtn(b item) bool { return b.icon != 0 && b.text == "" }

func btnWidth(b item, minW, pad int) int {
	if isIconBtn(b) {
		return px(38)
	}
	return max(minW, textW(b.font, b.text)+pad)
}

// rightRowMin places buttons from the right edge going left. If the row would reach minLeft (the title),
// the buttons are made narrower so nothing overlaps. It returns the left edge of the row.
func rightRowMin(it []item, right, y, h, gap, minLeft int, bs []item) ([]item, int) {
	fixed := isIconBtn
	widths := func(minW, pad int) ([]int, int) {
		ws := make([]int, len(bs))
		total := gap * (len(bs) - 1)
		for i, b := range bs {
			ws[i] = btnWidth(b, minW, pad)
			total += ws[i]
		}
		return ws, total
	}
	ws, total := widths(px(54), px(22))
	if minLeft > 0 && right-total < minLeft {
		ws, total = widths(px(34), px(12))
	}
	if minLeft > 0 && right-total < minLeft {
		// Still too wide: shrink the text buttons evenly (their labels get an ellipsis) so nothing reaches the title.
		flex, fix := 0, 0
		for i, b := range bs {
			if fixed(b) {
				fix += ws[i]
			} else {
				flex += ws[i]
			}
		}
		fit := max(right-minLeft-gap*(len(bs)-1)-fix, px(24)*len(bs))
		for i, b := range bs {
			if !fixed(b) {
				ws[i] = max(px(24), ws[i]*fit/max(flex, 1))
			}
		}
	}
	x := right
	for i := len(bs) - 1; i >= 0; i-- {
		bs[i].r = rc(x-ws[i], y, x, y+h)
		it = append(it, bs[i])
		x -= ws[i] + gap
	}
	return it, x + gap
}

func rightRow(it []item, right, y, h, gap int, bs []item) ([]item, int) {
	return rightRowMin(it, right, y, h, gap, 0, bs)
}

// leftRow places buttons from the left edge going right, shrinking them if the row would overflow.
func leftRow(it []item, left, right, y, h, gap int, bs []item) []item {
	ws := make([]int, len(bs))
	total := gap * (len(bs) - 1)
	for i, b := range bs {
		ws[i] = btnWidth(b, px(70), px(22))
		total += ws[i]
	}
	if total > right-left {
		total = gap * (len(bs) - 1)
		for i, b := range bs {
			ws[i] = btnWidth(b, px(34), px(12))
			total += ws[i]
		}
	}
	if total > right-left {
		sum, fit := total-gap*(len(bs)-1), right-left-gap*(len(bs)-1)
		for i := range ws {
			ws[i] = max(px(24), ws[i]*fit/max(sum, 1))
		}
	}
	x := left
	for i, b := range bs {
		b.r = rc(x, y, x+ws[i], y+h)
		it = append(it, b)
		x += ws[i] + gap
	}
	return it
}

func build(s snapshot) ([]item, int) {
	if s.help {
		return buildHelp(s)
	}
	if s.langPage {
		return buildLangPage(s)
	}
	var it []item
	W, pad, ip := panelW(false), px(16), px(14)
	y, bh := px(14), px(28)
	it = append(it, item{kind: kStar, r: rc(pad, y+px(1), pad+px(26), y+px(27))})
	it = append(it, item{kind: kText, r: rc(pad+px(34), y, pad+px(150), y+bh), text: "Lumistar", font: 1, flags: tLeft})
	theme := tx("light")
	if s.light {
		theme = tx("dark")
	}
	it, _ = rightRowMin(it, W-pad, y, bh, px(6), pad+px(34)+textW(1, "Lumistar")+px(12), []item{
		{kind: kButton, id: "snip", icon: 11, flags: tCenter},
		{kind: kButton, id: "night", on: s.night, text: tx("night"), flags: tCenter},
		{kind: kButton, id: "theme", text: theme, flags: tCenter},
		{kind: kButton, id: "quit", text: tx("quit"), flags: tCenter},
	})
	y += bh + px(14)

	// Brightness card
	n := max(len(s.disp), 1)
	ch := px(34) + n*px(48) + px(8)
	it = append(it, item{kind: kCard, r: rc(pad, y, W-pad, y+ch)})
	pw := 10*px(24) + 9*px(2)
	it = append(it, item{kind: kText, r: rc(pad+ip, y+px(10), W-pad-ip-pw-px(6), y+px(28)), text: tx("brightness"), font: 2, col: 1, flags: tLeft})
	it = append(it, presetItems("pb", W-pad-ip, y+px(6), func(v int) bool {
		if len(s.disp) == 0 {
			return false
		}
		for _, d := range s.disp {
			if d.val != v {
				return false
			}
		}
		return true
	})...)
	ry := y + px(34)
	switch {
	case !s.loaded:
		it = append(it, item{kind: kText, r: rc(pad+ip, ry, W-pad-ip, ry+px(40)), text: tx("looking"), col: 1, flags: tWrap})
	case len(s.disp) == 0:
		it = append(it, item{kind: kText, r: rc(pad+ip, ry, W-pad-ip, ry+px(44)), text: tx("nodisp"), col: 1, flags: tWrap})
	}
	for i, d := range s.disp {
		name := d.name
		if d.internal {
			name = tx("builtin")
		}
		it = append(it, rowItems(pad+ip, W-pad-ip, ry, name, d.val, "b"+strconv.Itoa(i))...)
		ry += px(48)
	}
	y += ch + px(12)

	// Volume card
	ch = px(34) + px(48) + px(28) + px(12)
	it = append(it, item{kind: kCard, r: rc(pad, y, W-pad, y+ch)})
	it = append(it, item{kind: kText, r: rc(pad+ip, y+px(10), W-pad-ip-pw-px(6), y+px(28)), text: tx("volume"), font: 2, col: 1, flags: tLeft})
	it = append(it, presetItems("pv", W-pad-ip, y+px(6), func(v int) bool { return s.vol == v })...)
	ry = y + px(34)
	it = append(it, rowItems(pad+ip, W-pad-ip, ry, tx("speakers"), s.vol, "v")...)
	it = leftRow(it, pad+ip, W-pad-ip, ry+px(48), px(28), px(6), []item{
		{kind: kButton, id: "mute", icon: 6, on: s.muted, flags: tCenter},
		{kind: kButton, id: "mic", icon: 1, on: s.micMuted, flags: tCenter},
		{kind: kButton, id: "awake", on: s.awake, text: tx("awake"), flags: tCenter},
		{kind: kButton, id: "langs", text: tx("languages"), flags: tCenter},
		{kind: kButton, id: "soundset", icon: 7, flags: tCenter},
	})
	y += ch + px(14)

	// Footer
	it = append(it, item{kind: kCheck, id: "auto", on: s.autostart, r: rc(pad, y, pad+px(18), y+px(18))})
	var left int
	it, left = rightRowMin(it, W-pad, y-px(5), px(28), px(6), pad+px(28)+px(90), []item{
		{kind: kButton, id: "help", text: tx("help"), flags: tCenter},
		{kind: kButton, id: "lock", text: tx("lock"), flags: tCenter},
		{kind: kButton, id: "off", text: tx("screenoff"), flags: tCenter},
	})
	it = append(it, item{kind: kText, r: rc(pad+px(28), y-px(2), left-px(8), y+px(20)), text: tx("launch"), col: 1, flags: tLeft})
	return it, y + px(20) + px(16)
}

const (
	langPages   = 4
	langPerPage = 16
)

// langGrid lays out langs[from:to] in two columns, in the exact order of the slice. Every button carries its flag.
// The English cell is split in two halves: EN US (left) and EN UK (right).
func langGrid(it []item, y, from, to int, s snapshot) ([]item, int) {
	W, pad := panelW(false), px(16)
	gap, rowH := px(8), px(38)
	colW := (W - 2*pad - gap) / 2
	for i := from; i < to; i++ {
		n := i - from
		x := pad + (n%2)*(colW+gap)
		yy := y + (n/2)*(rowH+gap)
		if langs[i].code == "en" {
			half := (colW - px(4)) / 2
			it = append(it, item{kind: kButton, id: "L" + strconv.Itoa(enUSIndex), on: s.lang == enUSIndex, text: langs[enUSIndex].name, flag: langs[enUSIndex].flag, flags: tCenter, r: rc(x, yy, x+half, yy+rowH)})
			it = append(it, item{kind: kButton, id: "L" + strconv.Itoa(i), on: i == s.lang, text: langs[i].name, flag: langs[i].flag, flags: tCenter, r: rc(x+half+px(4), yy, x+colW, yy+rowH)})
			continue
		}
		it = append(it, item{kind: kButton, id: "L" + strconv.Itoa(i), on: i == s.lang, text: langs[i].name, flag: langs[i].flag, flags: tCenter, r: rc(x, yy, x+colW, yy+rowH)})
	}
	rows := (to - from + 1) / 2
	return it, y + rows*(rowH+gap) + px(8)
}

// buildLangPage draws one of the 3 language pages. Page 1 has Back + Next Page, page 2 has Previous Page + Next Page,
// page 3 has Previous Page. Button widths are measured and shrunk if needed so nothing can overlap the title or each other.
func buildLangPage(s snapshot) ([]item, int) {
	pg := s.langPg
	var it []item
	W, pad := panelW(false), px(16)
	y, bh := px(14), px(28)
	leftID, leftText := "pgprev", tx("prevpage")
	if pg == 0 {
		leftID, leftText = "back", tx("back")
	}
	rightText := tx("nextpage")
	lw := max(px(64), textW(0, leftText)+px(24))
	rw := 0
	if pg < langPages-1 {
		rw = max(px(64), textW(0, rightText)+px(24))
	}
	if avail := W - 2*pad - px(110); lw+rw > avail {
		lw, rw = lw*avail/(lw+rw), rw*avail/(lw+rw)
	}
	it = append(it, item{kind: kButton, id: leftID, r: rc(pad, y, pad+lw, y+bh), text: leftText, flags: tCenter})
	if rw > 0 {
		it = append(it, item{kind: kButton, id: "pgnext", r: rc(W-pad-rw, y, W-pad, y+bh), text: rightText, flags: tCenter})
	}
	title := tx("languages") + "  " + strconv.Itoa(pg+1) + "/" + strconv.Itoa(langPages)
	it = append(it, item{kind: kText, r: rc(pad+lw+px(8), y, W-pad-rw-px(8), y+bh), text: title, font: 1, flags: tCenter})
	y += bh + px(14)
	// The last page is shorter (6 languages) and the final entry of langs is the EN US variant, so clamp the range.
	return langGrid(it, y, pg*langPerPage, min((pg+1)*langPerPage, len(langs)-1), s)
}

func rowItems(l, r, y int, name string, val int, id string) []item {
	return []item{
		{kind: kText, r: rc(l, y, r-px(40), y+px(20)), text: name, flags: tLeft},
		{kind: kText, r: rc(r-px(40), y, r, y+px(20)), text: strconv.Itoa(val), col: 1, flags: tRight},
		{kind: kSlider, id: id, r: rc(l, y+px(24), r, y+px(44))},
	}
}

// ---------------------------------------------------------------- painting

type palette struct{ bg, surface, text, muted, accent, strong, track, btn uint32 }

var (
	dark   = palette{0x16121f, 0x1f1930, 0xf1edfa, 0xa79dc2, 0xa78bfa, 0x8b5cf6, 0x3a3052, 0x2d2542}
	bright = palette{0xffffff, 0xf5f2fc, 0x1c1530, 0x6a5f87, 0x7c3aed, 0x6d28d9, 0xddd5f2, 0xe6e0f5}
)

func bgr(c uint32) uintptr { return uintptr((c&0xFF)<<16 | c&0xFF00 | (c>>16)&0xFF) }

type painter struct {
	g  uintptr
	br map[uint32]uintptr
}

func (p *painter) brush(c uint32) uintptr {
	if b, ok := p.br[c]; ok {
		return b
	}
	var b uintptr
	call(pGdipFill, uintptr(0xFF000000|c), ptr(&b))
	p.br[c] = b
	return b
}
func (p *painter) box(x, y, w, h int, c uint32) {
	call(pGdipRect, p.g, p.brush(c), iu(x), iu(y), iu(w), iu(h))
}
func (p *painter) dot(x, y, w, h int, c uint32) {
	call(pGdipEllipse, p.g, p.brush(c), iu(x), iu(y), iu(w), iu(h))
}
func (p *painter) round(x, y, w, h, r int, c uint32) {
	r = min(r, min(w, h)/2)
	p.box(x+r, y, w-2*r, h, c)
	p.box(x, y+r, w, h-2*r, c)
	p.dot(x, y, 2*r, 2*r, c)
	p.dot(x+w-2*r, y, 2*r, 2*r, c)
	p.dot(x, y+h-2*r, 2*r, 2*r, c)
	p.dot(x+w-2*r, y+h-2*r, 2*r, 2*r, c)
}
func (p *painter) star(r rect, c uint32) {
	cx, cy := float64(r.l+r.r)/2, float64(r.t+r.b)/2+0.5
	R := float64(r.r-r.l) / 2
	var pts [10]point
	for i := range pts {
		rad := R
		if i%2 == 1 {
			rad = R * 0.45
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		pts[i] = point{int32(math.Round(cx + rad*math.Cos(a))), int32(math.Round(cy + rad*math.Sin(a)))}
	}
	call(pGdipPolygon, p.g, p.brush(c), uintptr(unsafe.Pointer(&pts[0])), 10, 0)
}

func sliderGeometry(r rect) (x0, x1, cy int) {
	tr := px(8)
	return int(r.l) + tr, int(r.r) - tr, int(r.t+r.b) / 2
}

func paint() {
	s := snap()
	items, h := build(s)
	w := panelW(s.help)
	pal := dark
	if s.light {
		pal = bright
	}
	var ps paintStruct
	hdc := call(pBeginPaint, hwnd, ptr(&ps))
	mem := call(pCompatDC, hdc)
	bmp := call(pCompatBmp, hdc, uintptr(w), uintptr(h))
	old := call(pSelect, mem, bmp)

	bg := call(pSolidBrush, bgr(pal.bg))
	full := rc(0, 0, w, h)
	call(pFillRect, mem, ptr(&full), bg)
	call(pDeleteObj, bg)

	var g uintptr
	call(pGdipGraphics, mem, ptr(&g))
	call(pGdipSmooth, g, 4)
	p := &painter{g: g, br: map[uint32]uintptr{}}
	for _, it := range items {
		x, y, ww, hh := int(it.r.l), int(it.r.t), int(it.r.r-it.r.l), int(it.r.b-it.r.t)
		switch it.kind {
		case kCard:
			p.round(x, y, ww, hh, px(12), pal.surface)
		case kStar:
			call(pGdipInterp, p.g, 7)
			call(pGdipDrawImg, p.g, starImg, iu(x), iu(y), iu(ww), iu(hh))
		case kButton:
			c := pal.btn
			if it.on {
				c = pal.strong
			}
			if (it.id == "mic" || it.id == "mute") && it.on {
				c = 0xED4245 // muted microphone: the whole button turns red, like Discord
			}
			rad := px(8)
			if ww < px(26) {
				rad = px(5)
			}
			p.round(x, y, ww, hh, rad, c)
			p.icon(it, s.light, pal, c)
			if it.flag != "" {
				p.flag(it)
			}
		case kBadge:
			p.icon(it, s.light, pal, pal.surface)
		case kCheck:
			if it.on {
				p.round(x, y, ww, hh, px(4), pal.strong)
				p.round(x+px(5), y+px(5), ww-px(10), hh-px(10), px(2), 0xffffff)
			} else {
				p.round(x, y, ww, hh, px(4), pal.muted)
				p.round(x+px(2), y+px(2), ww-px(4), hh-px(4), px(3), pal.bg)
			}
		case kSlider:
			v := sliderValue(s, it.id)
			x0, x1, cy := sliderGeometry(it.r)
			th := px(6)
			cx := x0 + v*(x1-x0)/100
			fill := pal.strong
			if it.id == "v" && s.muted {
				fill = pal.muted
			}
			p.round(x0, cy-th/2, x1-x0, th, th/2, pal.track)
			p.round(x0, cy-th/2, cx-x0, th, th/2, fill)
			tr := px(8)
			p.dot(cx-tr, cy-tr, 2*tr, 2*tr, fill)
			p.dot(cx-tr+px(2), cy-tr+px(2), 2*tr-px(4), 2*tr-px(4), 0xffffff)
		}
	}
	call(pGdipDelGfx, g)
	for _, b := range p.br {
		call(pGdipDelBrush, b)
	}

	call(pBkMode, mem, 1)
	for _, it := range items {
		if it.text == "" {
			continue
		}
		col := pal.text
		switch it.col {
		case 1:
			col = pal.muted
		case 3:
			col = pal.accent
		case 4: // Claude orange, the colour of the Claude logo
			col = 0xD97757
		}
		if it.kind == kButton && it.on {
			col = 0xffffff
		}
		r := it.r
		if it.flag != "" {
			r.l += int32(px(30)) // leave room for the flag on the left
		}
		sp := u16(it.text)
		call(pSelect, mem, fonts[it.font])
		call(pTextColor, mem, bgr(col))
		call(pDrawText, mem, uintptr(unsafe.Pointer(sp)), ^uintptr(0), ptr(&r), it.flags)
		runtime.KeepAlive(sp)
	}
	call(pBitBlt, hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, 0x00CC0020)
	call(pSelect, mem, old)
	call(pDeleteObj, bmp)
	call(pDeleteDC, mem)
	call(pEndPaint, hwnd, ptr(&ps))
}

func sliderValue(s snapshot, id string) int {
	if id == "v" {
		return s.vol
	}
	if i, err := strconv.Atoi(id[1:]); err == nil && i < len(s.disp) {
		return s.disp[i].val
	}
	return 0
}

// ---------------------------------------------------------------- input

func lparam(lp uintptr) (int, int) { return int(int16(lp & 0xFFFF)), int(int16((lp >> 16) & 0xFFFF)) }

func mouseDown(lp uintptr) {
	x, y := lparam(lp)
	s := snap()
	items, _ := build(s)
	for _, it := range items {
		if it.kind != kSlider && it.kind != kButton && it.kind != kCheck && it.kind != kLink {
			continue
		}
		r := it.r
		if it.kind == kSlider {
			r.t -= int32(px(6))
			r.b += int32(px(4))
		}
		if it.kind == kCheck {
			r.r += int32(px(130))
		}
		if x < int(r.l) || x >= int(r.r) || y < int(r.t) || y >= int(r.b) {
			continue
		}
		switch {
		case it.kind == kSlider:
			drag = it.id
			call(pSetCapture, hwnd)
			setSlider(it, x)
		case it.id == "quit":
			quit()
		case it.id == "theme":
			mu.Lock()
			light = !light
			l := light
			mu.Unlock()
			saveTheme(l)
		case it.id == "mute":
			mu.Lock()
			muted = !muted
			m := muted
			mu.Unlock()
			go func() {
				defer guard()
				setMute(m)
			}()
		case strings.HasPrefix(it.id, "pb") || strings.HasPrefix(it.id, "pv"):
			applyPreset(it.id)
		case it.id == "help" || it.id == "back":
			mu.Lock()
			help = it.id == "help"
			langPage, langPg, helpScroll = false, 0, 0
			mu.Unlock()
			relayout()
		case it.id == "langs":
			mu.Lock()
			langPage, help, langPg = true, false, 0
			mu.Unlock()
			relayout()
		case it.id == "pgnext" || it.id == "pgprev":
			mu.Lock()
			if it.id == "pgnext" && langPg < langPages-1 {
				langPg++
			} else if it.id == "pgprev" && langPg > 0 {
				langPg--
			}
			mu.Unlock()
			relayout()
		case len(it.id) > 1 && it.id[0] == 'L':
			selectLang(it.id)
		case it.id == "soundset":
			openURL("ms-settings:sound")
		case it.id == "snip":
			hidePanel()
			openURL("ms-screenclip:") // Windows screenshot overlay; no keys are sent
		case it.id == "mic":
			toggleMic()
		case it.id == "lock":
			lockPC()
		case it.id == "link":
			openURL(portfolioURL)
		case it.id == "linkclaude" || it.id == "claudeicon":
			openURL(claudeURL)
		case it.id == "ghicon":
			openURL(repoURL)
		case it.id == "night":
			toggleNight()
		case it.id == "awake":
			toggleAwake()
		case it.id == "off":
			screenOff()
		case it.id == "auto":
			mu.Lock()
			autostart = !autostart
			a := autostart
			mu.Unlock()
			go func() {
				defer guard()
				setAutostart(a)
			}()
		}
		call(pInvalidate, hwnd, 0, 0)
		return
	}
}

func mouseMove(lp uintptr) {
	x, _ := lparam(lp)
	items, _ := build(snap())
	for _, it := range items {
		if it.kind == kSlider && it.id == drag {
			setSlider(it, x)
			call(pInvalidate, hwnd, 0, 0)
		}
	}
}

func setSlider(it item, x int) {
	x0, x1, _ := sliderGeometry(it.r)
	v := min(max((x-x0)*100/max(x1-x0, 1), 0), 100)
	if it.id == "v" {
		mu.Lock()
		vol = v
		mu.Unlock()
		latest("v", v, setVolume)
		return
	}
	i, err := strconv.Atoi(it.id[1:])
	mu.Lock()
	if err != nil || i >= len(disp) {
		mu.Unlock()
		return
	}
	disp[i].val = v
	night = false
	mu.Unlock()
	latest(it.id, v, func(v int) { setBrightness(i, v) })
}

// ---------------------------------------------------------------- brightness

type physMon struct {
	h    uintptr
	desc [128]uint16
}
type monitor struct {
	h        uintptr
	name     string
	min, max uint32
}

var (
	dispMu sync.Mutex
	bufs   [][]physMon
	mons   []monitor
	hmons  []uintptr
	enumCB = syscall.NewCallback(func(hm, _, _, _ uintptr) uintptr { hmons = append(hmons, hm); return 1 })
)

func pct(m monitor) (int, bool) {
	var lo, cur, hi uint32
	if call(pGetBright, m.h, ptr(&lo), ptr(&cur), ptr(&hi)) == 0 || hi <= lo {
		return 0, false
	}
	return int((cur - lo) * 100 / (hi - lo)), true
}

func enumerateDisplays() {
	dispMu.Lock()
	for _, b := range bufs {
		call(pDestroyPhys, uintptr(len(b)), ptr(&b[0]))
	}
	bufs, mons, hmons = nil, nil, nil
	call(pEnumMonitors, 0, 0, enumCB, 0)
	for _, hm := range hmons {
		var n uint32
		if call(pNumPhys, hm, ptr(&n)) == 0 || n == 0 {
			continue
		}
		buf := make([]physMon, n)
		if call(pGetPhys, hm, uintptr(n), ptr(&buf[0])) == 0 {
			continue
		}
		bufs = append(bufs, buf)
		for _, pm := range buf {
			m := monitor{h: pm.h, name: syscall.UTF16ToString(pm.desc[:])}
			var lo, cur, hi uint32
			if call(pGetBright, pm.h, ptr(&lo), ptr(&cur), ptr(&hi)) != 0 && hi > lo {
				m.min, m.max = lo, hi
				mons = append(mons, m)
			}
		}
	}
	var list []dispState
	for i, m := range mons {
		v, _ := pct(m)
		name := m.name
		if name == "" {
			name = fmt.Sprintf("Display %d", i+1)
		}
		list = append(list, dispState{id: fmt.Sprintf("d%d", i), name: name, val: v, mon: i})
	}
	dispMu.Unlock()

	if v, ok := internalGet(); ok {
		list = append([]dispState{{id: "internal", name: "Built-in display", val: v, internal: true}}, list...)
	}
	mu.Lock()
	disp = list
	mu.Unlock()
}

func readValues() {
	mu.Lock()
	cur := append([]dispState(nil), disp...)
	mu.Unlock()
	for i, d := range cur {
		if d.internal {
			if v, ok := internalGet(); ok {
				cur[i].val = v
			}
			continue
		}
		dispMu.Lock()
		if d.mon < len(mons) {
			if v, ok := pct(mons[d.mon]); ok {
				cur[i].val = v
			}
		}
		dispMu.Unlock()
	}
	mu.Lock()
	if len(disp) == len(cur) {
		for i := range cur {
			disp[i].val = cur[i].val
		}
	}
	mu.Unlock()
}

func setBrightness(i, v int) {
	mu.Lock()
	if i >= len(disp) {
		mu.Unlock()
		return
	}
	d := disp[i]
	mu.Unlock()
	if d.internal {
		internalSet(v)
		return
	}
	dispMu.Lock()
	defer dispMu.Unlock()
	if d.mon < len(mons) {
		m := mons[d.mon]
		call(pSetBright, m.h, uintptr(m.min+uint32(v)*(m.max-m.min)/100))
	}
}

// ---------------------------------------------------------------- volume (Core Audio, raw COM)

var (
	clsidEnum = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidEnum   = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidEP     = guid{0x5CDF2C82, 0x841E, 0x4546, [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
	audioCh   = make(chan func())
)

func audioLoop() {
	runtime.LockOSThread()
	call(pCoInit, 0, 0)
	for f := range audioCh {
		func() {
			defer guard()
			f()
		}()
	}
}

func audioDo(f func(ep uintptr)) { audioDoFlow(0, f) }

// audioDoFlow runs f on the default endpoint: flow 0 = speakers (render), 1 = microphone (capture).
func audioDoFlow(flow uintptr, f func(ep uintptr)) {
	done := make(chan struct{})
	audioCh <- func() {
		defer close(done)
		var en, dev, ep uintptr
		if int32(call(pCoCreate, ptr(&clsidEnum), 0, 23, ptr(&iidEnum), ptr(&en))) < 0 {
			return
		}
		defer vcall(en, 2)
		if int32(vcall(en, 4, flow, 0, ptr(&dev))) < 0 {
			return
		}
		defer vcall(dev, 2)
		if int32(vcall(dev, 3, ptr(&iidEP), 23, 0, ptr(&ep))) < 0 {
			return
		}
		defer vcall(ep, 2)
		f(ep)
	}
	<-done
}

func vcall(obj, idx uintptr, args ...uintptr) uintptr {
	vt := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vt + idx*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func getVolume() (v int, m bool) {
	audioDo(func(ep uintptr) {
		var f float32
		var mu int32
		vcall(ep, 9, ptr(&f))
		vcall(ep, 15, ptr(&mu))
		v, m = int(math.Round(float64(f)*100)), mu != 0
	})
	return
}

func setVolume(v int) {
	audioDo(func(ep uintptr) { vcall(ep, 7, uintptr(math.Float32bits(float32(v)/100)), 0) })
}

func setMute(m bool) {
	audioDo(func(ep uintptr) {
		b := uintptr(0)
		if m {
			b = 1
		}
		vcall(ep, 14, b, 0)
	})
}

// ---------------------------------------------------------------- start-up

// Start-up entry: written straight to the registry with the Windows API, no child process.
const (
	runKeyPath      = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName    = "Lumistar"
	hkeyCurrentUser = uintptr(0x80000001)
	keyQueryValue   = uintptr(0x0001)
	keySetValue     = uintptr(0x0002)
	regSZ           = uintptr(1)
)

var (
	pRegCreate = advapi32.NewProc("RegCreateKeyExW")
	pRegOpen   = advapi32.NewProc("RegOpenKeyExW")
	pRegSet    = advapi32.NewProc("RegSetValueExW")
	pRegQuery  = advapi32.NewProc("RegQueryValueExW")
	pRegDelete = advapi32.NewProc("RegDeleteValueW")
	pRegClose  = advapi32.NewProc("RegCloseKey")
)

// regOK reports ERROR_SUCCESS. Only the low 32 bits of a registry call's result are meaningful.
func regOK(r uintptr) bool { return int32(r) == 0 }

func autostartOn() bool {
	keyPath, name := u16(runKeyPath), u16(runValueName)
	var hk uintptr
	if !regOK(call(pRegOpen, hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, keyQueryValue, ptr(&hk))) {
		return false
	}
	found := regOK(call(pRegQuery, hk, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0))
	call(pRegClose, hk)
	runtime.KeepAlive(keyPath)
	runtime.KeepAlive(name)
	return found
}

func setAutostart(on bool) {
	keyPath, name := u16(runKeyPath), u16(runValueName)
	var hk uintptr
	defer runtime.KeepAlive(keyPath)
	defer runtime.KeepAlive(name)
	if !on {
		if regOK(call(pRegOpen, hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, keySetValue, ptr(&hk))) {
			call(pRegDelete, hk, uintptr(unsafe.Pointer(name)))
			call(pRegClose, hk)
		}
		return
	}
	exe, _ := os.Executable()
	data := syscall.StringToUTF16(`"` + exe + `"`)
	if regOK(call(pRegCreate, hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, 0, 0, keySetValue, 0, ptr(&hk), 0)) {
		call(pRegSet, hk, uintptr(unsafe.Pointer(name)), 0, regSZ, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
		call(pRegClose, hk)
	}
	runtime.KeepAlive(data)
}

// ---------------------------------------------------------------- extras

var pExecState = kernel32.NewProc("SetThreadExecutionState")

// Night: dims every display to a low level; press again to restore the previous levels.
func toggleNight() {
	mu.Lock()
	n := len(disp)
	if !night {
		nightSaved = make([]int, n)
		for i := range disp {
			nightSaved[i], disp[i].val = disp[i].val, nightLevel
		}
	} else if len(nightSaved) == n {
		for i := range disp {
			disp[i].val = nightSaved[i]
		}
	}
	night = !night
	vals := make([]int, n)
	for i := range disp {
		vals[i] = disp[i].val
	}
	mu.Unlock()
	for i, v := range vals {
		i := i
		latest("b"+strconv.Itoa(i), v, func(x int) { setBrightness(i, x) })
	}
}

// Stay awake: stops the PC and screens from sleeping until switched off or Lumistar quits.
func toggleAwake() {
	mu.Lock()
	awake = !awake
	a := awake
	mu.Unlock()
	flags := uintptr(0x80000000) // ES_CONTINUOUS
	if a {
		flags |= 0x3 // ES_SYSTEM_REQUIRED | ES_DISPLAY_REQUIRED
	}
	call(pExecState, flags)
}

// Screen off: switches the monitors off; move the mouse to wake them.
func screenOff() {
	hidePanel()
	go func() {
		time.Sleep(500 * time.Millisecond)
		call(pPostMessage, 0xFFFF, 0x112, 0xF170, 2)
	}()
}

// ---------------------------------------------------------------- help page, star artwork, link

//go:embed star.raw
var starRaw []byte

var (
	starImg      uintptr
	pGetDC       = user32.NewProc("GetDC")
	pReleaseDC   = user32.NewProc("ReleaseDC")
	pTextExt     = gdi32.NewProc("GetTextExtentPoint32W")
	pShellExec   = shell32.NewProc("ShellExecuteW")
	pGdipBitmap  = gdiplus.NewProc("GdipCreateBitmapFromScan0")
	pGdipDrawImg = gdiplus.NewProc("GdipDrawImageRectI")
	pGdipInterp  = gdiplus.NewProc("GdipSetInterpolationMode")
	measureCache = map[string]int{}
)

const portfolioURL = "https://guns.lol/fuwifur"

func panelW(help bool) int {
	if help {
		return px(520)
	}
	return px(400)
}

func relayout() {
	st := snap()
	_, h := build(st)
	w := panelW(st.help)
	x := max(curX+curW-w, 8)
	y := max(curY+curH-h, 8)
	call(pSetWindowPos, hwnd, 0, iu(x), iu(y), uintptr(w), iu(h), 0x14)
	curX, curY, curW, curH = x, y, w, h
	call(pInvalidate, hwnd, 0, 0)
}

func openURL(u string) {
	verb, url := u16("open"), u16(u)
	call(pShellExec, 0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(url)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(url)
}

// measure returns the wrapped height of text in the given font and width.
func measure(font int, s string, w int) int {
	key := fmt.Sprintf("%d|%d|%s", font, w, s)
	if v, ok := measureCache[key]; ok {
		return v
	}
	dc := call(pGetDC, 0)
	old := call(pSelect, dc, fonts[font])
	r := rect{0, 0, int32(w), 0}
	sp := u16(s)
	call(pDrawText, dc, uintptr(unsafe.Pointer(sp)), ^uintptr(0), ptr(&r), 0x10|0x800|0x400)
	runtime.KeepAlive(sp)
	call(pSelect, dc, old)
	call(pReleaseDC, 0, dc)
	measureCache[key] = int(r.b)
	return int(r.b)
}

// textW returns the exact single-line width of text (trailing spaces included).
var textWCache = map[string]int{}

func textW(font int, s string) int {
	key := strconv.Itoa(font) + "|" + s
	if v, ok := textWCache[key]; ok {
		return v
	}
	dc := call(pGetDC, 0)
	old := call(pSelect, dc, fonts[font])
	u := syscall.StringToUTF16(s)
	var sz [2]int32
	call(pTextExt, dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), ptr(&sz))
	runtime.KeepAlive(u)
	call(pSelect, dc, old)
	call(pReleaseDC, 0, dc)
	textWCache[key] = int(sz[0])
	return int(sz[0])
}

func buildHelp(s snapshot) ([]item, int) {
	var it []item
	W, pad, ip := panelW(true), px(16), px(14)
	y, bh := px(14), px(28)
	bw := max(px(64), textW(0, tx("back"))+px(24))
	it = append(it, item{kind: kButton, id: "back", r: rc(pad, y, pad+bw, y+bh), text: tx("back"), flags: tCenter})
	it = append(it, item{kind: kText, r: rc(pad+bw+px(12), y, W-pad, y+bh), text: tx("helptitle"), font: 1, flags: tLeft})
	y += bh + px(12)

	intro := tx("intro")
	ih := measure(3, intro, W-2*pad)
	it = append(it, item{kind: kText, r: rc(pad, y, W-pad, y+ih), text: intro, font: 3, col: 1, flags: tWrap})
	y += ih + px(12)

	ic := func(n int) string { return "\x01" + string(rune('0'+n)) } // a row whose name is a picture
	rows := [][2]string{
		{tx("brightness"), tx("d_brightness")},
		{tx("volume"), tx("d_volume")},
		{ic(6), tx("d_mute")},
		{ic(1), tx("d_mic")},
		{tx("night"), tx("d_night")},
		{tx("awake"), tx("d_awake")},
		{tx("screenoff"), tx("d_screenoff")},
		{tx("light") + " / " + tx("dark"), tx("d_theme")},
		{tx("launch"), tx("d_launch")},
		{tx("lock"), tx("d_lock")},
		{tx("languages"), tx("d_languages")},
		{ic(11), tx("d_snip")},
		{ic(7), tx("d_sound")},
		{tx("quit"), tx("d_quit")},
	}
	nameW := px(112)
	descW := W - 2*pad - 2*ip - nameW - px(10)
	hs := make([]int, len(rows))
	total := 0
	for i, r := range rows {
		nh := measure(2, r[0], nameW)
		if strings.HasPrefix(r[0], "\x01") {
			nh = px(22)
		}
		hs[i] = max(nh, measure(3, r[1], descW))
		total += hs[i] + px(8)
	}
	ch := px(12) + total + px(2)
	it = append(it, item{kind: kCard, r: rc(pad, y, W-pad, y+ch)})
	ry := y + px(12)
	for i, r := range rows {
		if strings.HasPrefix(r[0], "\x01") {
			it = append(it, item{kind: kBadge, icon: int(r[0][1] - '0'), r: rc(pad+ip, ry, pad+ip+px(30), ry+px(22))})
		} else {
			it = append(it, item{kind: kText, r: rc(pad+ip, ry, pad+ip+nameW, ry+hs[i]), text: r[0], font: 2, col: 3, flags: tWrap})
		}
		it = append(it, item{kind: kText, r: rc(pad+ip+nameW+px(10), ry, W-pad-ip, ry+hs[i]), text: r[1], font: 3, flags: tWrap})
		ry += hs[i] + px(8)
	}
	y += ch + px(14)

	// Credits: [GitHub logo]  Credits: Made by fuwifur using Claude Sonnet 5.5  [Claude logo]
	// "fuwifur" opens the portfolio, "Claude" and the Claude logo open claude.ai, the GitHub logo opens the repository.
	p1, p2, p3, p4, p5 := "Credits: Made by ", "fuwifur", " using ", "Claude", " Sonnet 5.5 and Haiku 5.5"
	w1, w2, w3, w4, w5 := textW(3, p1), textW(2, p2), textW(3, p3), textW(2, p4), textW(3, p5)
	isz, igap, lh := px(26), px(10), px(18)
	x := (W - (isz + igap + w1 + w2 + w3 + w4 + w5 + igap + isz)) / 2
	iy := y + lh/2 - isz/2
	it = append(it, item{kind: kButton, id: "ghicon", icon: 2, r: rc(x, iy, x+isz, iy+isz)})
	x += isz + igap
	it = append(it, item{kind: kText, r: rc(x, y, x+w1+px(2), y+lh), text: p1, font: 3, col: 1, flags: tLeft})
	x += w1
	it = append(it, item{kind: kLink, id: "link", r: rc(x, y, x+w2+px(2), y+lh), text: p2, font: 2, col: 3, flags: tLeft})
	x += w2
	it = append(it, item{kind: kText, r: rc(x, y, x+w3+px(2), y+lh), text: p3, font: 3, col: 1, flags: tLeft})
	x += w3
	it = append(it, item{kind: kLink, id: "linkclaude", r: rc(x, y, x+w4+px(2), y+lh), text: p4, font: 2, col: 4, flags: tLeft})
	x += w4
	it = append(it, item{kind: kText, r: rc(x, y, x+w5+px(2), y+lh), text: p5, font: 3, col: 1, flags: tLeft})
	x += w5 + igap
	it = append(it, item{kind: kButton, id: "claudeicon", icon: 3, r: rc(x, iy, x+isz, iy+isz)})
	y += isz + px(8)
	it = append(it, item{kind: kButton, id: "link", r: rc(pad, y, W-pad, y+bh), text: "Open guns.lol/fuwifur", flags: tCenter})
	pageH := y + bh + px(16)
	if workH > 0 && pageH > workH { // taller than the screen: scroll with the mouse wheel
		helpScroll = min(max(helpScroll, 0), pageH-workH)
		for i := range it {
			it[i].r.t -= int32(helpScroll)
			it[i].r.b -= int32(helpScroll)
		}
		return it, workH
	}
	helpScroll = 0
	return it, pageH
}

// ---------------------------------------------------------------- presets

func presetItems(prefix string, right, y int, active func(int) bool) []item {
	size, gap := px(24), px(2)
	x := right - (10*size + 9*gap)
	var out []item
	for i := 1; i <= 10; i++ {
		v := i * 10
		out = append(out, item{kind: kButton, id: prefix + strconv.Itoa(v), font: 4, on: active(v), text: strconv.Itoa(v),
			r: rc(x, y, x+size, y+size), flags: tCenter})
		x += size + gap
	}
	return out
}

func applyPreset(id string) {
	v, err := strconv.Atoi(id[2:])
	if err != nil {
		return
	}
	if id[:2] == "pv" {
		mu.Lock()
		vol = v
		mu.Unlock()
		latest("v", v, setVolume)
		return
	}
	mu.Lock()
	n := len(disp)
	for i := range disp {
		disp[i].val = v
	}
	night = false
	mu.Unlock()
	for i := 0; i < n; i++ {
		i := i
		latest("b"+strconv.Itoa(i), v, func(x int) { setBrightness(i, x) })
	}
}

// ---------------------------------------------------------------- languages and translations

type language struct{ code, name, flag string }

// 64 languages in 4 pages of 16, each named in its own language with its country flag; a page is langs[p*16:(p+1)*16], read row by row,
// left | right. Page 1: European languages (the first cell is split into EN US | EN UK; the last six are Latin-script European
// languages, most spoken first). Pages 2-4: the most spoken languages worldwide, sorted by total speakers (most spoken first).
// The very last entry is the EN US variant (see enUSIndex).
var langs = []language{
	{"en", "EN UK", "gb"},
	{"de", "Deutsch", "de"},
	{"tr", "Türkçe", "tr"},
	{"az", "Azərbaycanca", "az"},
	{"es", "Español", "es"},
	{"fr", "Français", "fr"},
	{"pl", "Polski", "pl"},
	{"ru", "Русский", "ru"},
	{"pt", "Português", "pt"},
	{"it", "Italiano", "it"},
	{"nl", "Nederlands", "nl"},
	{"ro", "Română", "ro"},
	{"hu", "Magyar", "hu"},
	{"sv", "Svenska", "se"},
	{"cs", "Čeština", "cz"},
	{"ca", "Català", "es-ct"},
	{"zh", "中文", "cn"},
	{"hi", "हिन्दी", "in"},
	{"ar", "العربية", "sa"},
	{"bn", "বাংলা", "bd"},
	{"ur", "اردو", "pk"},
	{"id", "Bahasa Indonesia", "id"},
	{"sw", "Kiswahili", "tz"},
	{"ja", "日本語", "jp"},
	{"pa", "ਪੰਜਾਬੀ", "in"},
	{"mr", "मराठी", "in"},
	{"te", "తెలుగు", "in"},
	{"vi", "Tiếng Việt", "vn"},
	{"jv", "Basa Jawa", "id"},
	{"ms", "Bahasa Melayu", "my"},
	{"fil", "Filipino", "ph"},
	{"fa", "فارسی", "ir"},
	{"ko", "한국어", "kr"},
	{"ta", "தமிழ்", "in"},
	{"ha", "Hausa", "ng"},
	{"gu", "ગુજરાતી", "in"},
	{"th", "ไทย", "th"},
	{"kn", "ಕನ್ನಡ", "in"},
	{"ps", "پښتو", "af"},
	{"am", "አማርኛ", "et"},
	{"bho", "भोजपुरी", "in"},
	{"yo", "Yorùbá", "ng"},
	{"my", "မြန်မာ", "mm"},
	{"su", "Basa Sunda", "id"},
	{"uk", "Українська", "ua"},
	{"or", "ଓଡ଼ିଆ", "in"},
	{"ml", "മലയാളം", "in"},
	{"om", "Afaan Oromoo", "et"},
	{"uz", "Oʻzbekcha", "uz"},
	{"mai", "मैथिली", "in"},
	{"ne", "नेपाली", "np"},
	{"ig", "Igbo", "ng"},
	{"sd", "سنڌي", "pk"},
	{"ku", "Kurdî", "iq"},
	{"ceb", "Cebuano", "ph"},
	{"zu", "isiZulu", "za"},
	{"mg", "Malagasy", "mg"},
	{"so", "Soomaali", "so"},
	{"xh", "isiXhosa", "za"},
	{"af", "Afrikaans", "za"},
	{"si", "සිංහල", "lk"},
	{"km", "ខ្មែរ", "kh"},
	{"kk", "Қазақша", "kz"},
	{"el", "Ελληνικά", "gr"},
	{"enus", "EN US", "us"},
}

// enUSIndex is the EN US entry (the last one). US English is the default language.
var enUSIndex = len(langs) - 1
var lang = enUSIndex

// tx returns the panel text for key in the chosen language (English when a translation is missing).
func tx(key string) string {
	v := strs[key]
	if lang >= 0 && lang < len(v) && v[lang] != "" {
		return v[lang]
	}
	return v[0]
}

// Column order follows langs above (the last column is EN US). An empty entry means "not translated yet": tx falls back to English.
var strs = map[string][65]string{
	"brightness":   {"Brightness", "Helligkeit", "Parlaklık", "Parlaqlıq", "Brillo", "Luminosité", "Jasność", "Яркость", "Brilho", "Luminosità", "Helderheid", "Luminozitate", "Fényerő", "Ljusstyrka", "Jas", "Brillantor", "亮度", "चमक", "السطوع", "উজ্জ্বলতা", "روشنی", "Kecerahan", "Mwangaza", "明るさ", "ਚਮਕ", "चमक", "ప్రకాశం", "Độ sáng", "Padhang", "Kecerahan", "Liwanag", "روشنایی", "밝기", "பிரகாசம்", "Haske", "તેજ", "ความสว่าง", "ಹೊಳಪು", "روښنایی", "ብሩህነት", "", "Ìmọ́lẹ̀", "တောက်ပမှု", "", "Яскравість", "", "തെളിച്ചം", "", "Yorqinlik", "", "चमक", "Ìhè", "", "Ronahî", "", "Ukukhanya", "", "Iftiin", "Ukukhanya", "Helderheid", "දීප්තිය", "ពន្លឺ", "Жарықтық", "Φωτεινότητα", ""},
	"volume":       {"Volume", "Lautstärke", "Ses", "Səs", "Volumen", "Volume", "Głośność", "Громкость", "Volume", "Volume", "Volume", "Volum", "Hangerő", "Volym", "Hlasitost", "Volum", "音量", "आवाज़", "مستوى الصوت", "ভলিউম", "آواز", "Volume", "Sauti", "音量", "ਆਵਾਜ਼", "आवाज", "వాల్యూమ్", "Âm lượng", "Volume", "Kelantangan", "Lakas ng tunog", "بلندی صدا", "볼륨", "ஒலியளவு", "Ƙarar sauti", "વોલ્યુમ", "ระดับเสียง", "ವಾಲ್ಯೂಮ್", "غږ", "ድምፅ", "", "Ohùn", "အသံ", "", "Гучність", "", "ശബ്ദം", "", "Ovoz balandligi", "", "आवाज", "Mgbapụta ụda", "", "Deng", "", "Ivolumu", "", "Cod", "Ivolumu", "Volume", "හඬ පරිමාව", "កម្រិតសំឡេង", "Дыбыс деңгейі", "Ένταση", ""},
	"speakers":     {"Speakers", "Lautsprecher", "Hoparlörler", "Dinamiklər", "Altavoces", "Haut-parleurs", "Głośniki", "Динамики", "Alto-falantes", "Altoparlanti", "Luidsprekers", "Difuzoare", "Hangszórók", "Högtalare", "Reproduktory", "Altaveus", "扬声器", "स्पीकर", "مكبرات الصوت", "স্পিকার", "اسپیکر", "Speaker", "Vipaza sauti", "スピーカー", "ਸਪੀਕਰ", "स्पीकर्स", "స్పీకర్లు", "Loa", "Speaker", "Pembesar suara", "Mga speaker", "بلندگوها", "스피커", "ஸ்பீக்கர்கள்", "Lasifika", "સ્પીકર્સ", "ลำโพง", "ಸ್ಪೀಕರ್‌ಗಳು", "لوډ سپیکرونه", "ድምጽ ማጉያዎች", "", "Àwọn agbóhùnsáfẹ́fẹ́", "စပီကာများ", "", "Динаміки", "", "സ്പീക്കറുകൾ", "", "Karnaylar", "", "स्पिकरहरू", "Ndị na-ekwu okwu", "", "Dengdêr", "", "Izipikha", "", "Sameecado", "Izithethi", "Luidsprekers", "විකාශක", "ឧបករណ៍បំពង្រីកសំឡេង", "Динамиктер", "Ηχεία", ""},
	"mute":         {"Mute", "Stumm", "Sessiz", "Səssiz", "Silenciar", "Muet", "Wycisz", "Без звука", "Silenciar", "Muto", "Dempen", "Dezactivează sunetul", "Némítás", "Ljud av", "Ztlumit", "Silencia", "静音", "म्यूट", "كتم", "মিউট", "خاموش", "Bisukan", "Nyamazisha", "ミュート", "ਮਿਊਟ", "म्यूट", "మ్యూట్", "Tắt tiếng", "Bisu", "Senyap", "I-mute", "بی‌صدا", "음소거", "ஒலியடக்கு", "Rufe sauti", "મ્યૂટ", "ปิดเสียง", "ಮ್ಯೂಟ್", "غږ بند کړئ", "ድምጽ አጥፋ", "", "Pa ohùn mọ́lẹ̀", "အသံတိတ်", "", "Вимкнути звук", "", "നിശബ്ദമാക്കുക", "", "Ovozsiz", "", "म्युट", "Mechie ụda", "", "Bêdeng bike", "", "Thulisa", "", "Aamusi", "Thulisa", "Demp", "නිහඬ කරන්න", "បិទសំឡេង", "Дыбысты өшіру", "Σίγαση", ""},
	"unmute":       {"Unmute", "Ton an", "Sesi aç", "Səsi aç", "Activar", "Son", "Włącz", "Включить", "Ativar som", "Riattiva", "Dempen opheffen", "Activează sunetul", "Némítás feloldása", "Ljud på", "Zrušit ztlumení", "Activa el so", "取消静音", "अनम्यूट", "إلغاء الكتم", "আনমিউট", "آواز کھولیں", "Bunyikan", "Rudisha sauti", "ミュート解除", "ਅਨਮਿਊਟ", "अनम्यूट", "అన్‌మ్యూట్", "Bật tiếng", "Nyuarakake", "Nyahsenyap", "I-unmute", "پخش صدا", "음소거 해제", "ஒலியை இயக்கு", "Buɗe sauti", "અનમ્યૂટ", "เปิดเสียง", "ಅನ್‌ಮ್ಯೂಟ್", "غږ پرانیزئ", "ድምጽ አብራ", "", "Ṣí ohùn", "အသံပြန်ဖွင့်", "", "Увімкнути звук", "", "ശബ്ദം പുനഃസ്ഥാപിക്കുക", "", "Ovozni yoqish", "", "अनम्युट", "Mepee ụda", "", "Dengê veke", "", "Susa ukuthula", "", "Dhaqaaji codka", "Susa ukuthula", "Hef demping op", "නිහඬතාව ඉවත් කරන්න", "បើកសំឡេង", "Дыбысты қосу", "Κατάργηση σίγασης", ""},
	"night":        {"Night", "Nacht", "Gece", "Gecə", "Noche", "Nuit", "Noc", "Ночь", "Noite", "Notte", "Nacht", "Noapte", "Éjszaka", "Natt", "Noc", "Nit", "夜间", "रात", "ليل", "রাত", "رات", "Malam", "Usiku", "夜間", "ਰਾਤ", "रात्र", "రాత్రి", "Ban đêm", "Wengi", "Malam", "Gabi", "شب", "야간", "இரவு", "Dare", "રાત", "กลางคืน", "ರಾತ್ರಿ", "شپه", "ሌሊት", "", "Òru", "ညဘက်", "", "Ніч", "", "രാത്രി", "", "Tun", "", "रात", "Abalị", "", "Şev", "", "Ubusuku", "", "Habeen", "Ubusuku", "Nag", "රාත්‍රිය", "ពេលយប់", "Түн", "Νύχτα", ""},
	"light":        {"Light", "Hell", "Açık", "Açıq", "Claro", "Clair", "Jasny", "Светлая", "Claro", "Chiaro", "Licht", "Luminos", "Világos", "Ljust", "Světlý", "Clar", "浅色", "हल्का", "فاتح", "হালকা", "روشن", "Terang", "Mwanga", "ライト", "ਲਾਈਟ", "लाइट", "లైట్", "Sáng", "Cerah", "Cerah", "Maliwanag", "روشن", "라이트", "ஒளி", "Mai haske", "હળવું", "สว่าง", "ಲೈಟ್", "روښانه", "ብሩህ", "", "Ìmọ́lẹ̀", "အလင်း", "", "Світла", "", "ലൈറ്റ്", "", "Och", "", "उज्यालो", "Ọcha", "", "Ronak", "", "Okukhanyayo", "", "Iftiin", "Okukhanyayo", "Lig", "ආලෝක", "ភ្លឺ", "Ашық", "Φωτεινό", ""},
	"dark":         {"Dark", "Dunkel", "Koyu", "Tünd", "Oscuro", "Sombre", "Ciemny", "Тёмная", "Escuro", "Scuro", "Donker", "Întunecat", "Sötét", "Mörkt", "Tmavý", "Fosc", "深色", "गहरा", "داكن", "গাঢ়", "گہرا", "Gelap", "Giza", "ダーク", "ਡਾਰਕ", "डार्क", "డార్క్", "Tối", "Peteng", "Gelap", "Madilim", "تیره", "다크", "இருள்", "Duhu", "ઘેરું", "มืด", "ಡಾರ್ಕ್", "تیاره", "ጨለማ", "", "Òkùnkùn", "အမှောင်", "", "Темна", "", "ഡാർക്ക്", "", "Qora", "", "अँध्यारो", "Ọchịchịrị", "", "Tarî", "", "Okumnyama", "", "Mugdi", "Okumnyama", "Donker", "අඳුරු", "ងងឹត", "Қараңғы", "Σκούρο", ""},
	"quit":         {"Quit", "Beenden", "Çık", "Çıx", "Salir", "Quitter", "Zamknij", "Выход", "Sair", "Esci", "Afsluiten", "Ieșire", "Kilépés", "Avsluta", "Ukončit", "Surt", "退出", "बंद करें", "خروج", "বন্ধ", "بند کریں", "Keluar", "Toka", "終了", "ਬਾਹਰ ਨਿਕਲੋ", "बंद करा", "నిష్క్రమించు", "Thoát", "Metu", "Keluar", "Lumabas", "خروج", "종료", "வெளியேறு", "Fita", "બહાર નીકળો", "ออก", "ನಿರ್ಗಮಿಸಿ", "وتل", "ውጣ", "", "Jáde", "ထွက်မည်", "", "Вихід", "", "പുറത്തുകടക്കുക", "", "Chiqish", "", "बाहिर निस्कनुहोस्", "Pụọ", "", "Derkeve", "", "Phuma", "", "Ka bax", "Phuma", "Verlaat", "ඉවත් වන්න", "ចេញ", "Шығу", "Έξοδος", ""},
	"awake":        {"Stay awake", "Wach halten", "Uyanık tut", "Oyaq saxla", "Mantener activo", "Rester actif", "Nie usypiaj", "Не засыпать", "Manter ativo", "Mantieni attivo", "Wakker houden", "Menține activ", "Ébren tartás", "Håll vaken", "Udržet vzhůru", "Mantén actiu", "保持唤醒", "जागृत रखें", "إبقاء مستيقظاً", "জাগিয়ে রাখুন", "جاگتے رکھیں", "Tetap aktif", "Baki macho", "スリープ防止", "ਜਾਗਦੇ ਰੱਖੋ", "जागे ठेवा", "మేల్కొని ఉంచు", "Luôn bật", "Tetep melek", "Kekal berjaga", "Manatiling gising", "بیدار نگه‌دار", "화면 켜짐 유지", "விழித்திருக்கவும்", "Ci gaba da farke", "જાગતા રાખો", "ไม่ให้หลับ", "ಎಚ್ಚರವಾಗಿಡಿ", "ویښ وساتئ", "ንቁ አድርግ", "", "Jẹ́ kó wà lójúfò", "နိုးကြားနေမည်", "", "Не засинати", "", "ഉണർന്നിരിക്കുക", "", "Uyg‘oq saqlash", "", "जागा राख्नुहोस्", "Debe ya na-arụ ọrụ", "", "Hişyar bihêle", "", "Hlala uvukile", "", "Ha seexan", "Hlala uphapheme", "Hou wakker", "අවදියෙන් තබන්න", "រក្សាឱ្យភ្ញាក់", "Ояу ұстау", "Μόνιμη ενεργοποίηση", ""},
	"screenoff":    {"Screen off", "Bildschirm aus", "Ekranı kapat", "Ekranı söndür", "Apagar pantalla", "Éteindre l'écran", "Wyłącz ekran", "Выкл. экран", "Desligar tela", "Spegni schermo", "Scherm uit", "Oprește ecranul", "Képernyő ki", "Skärm av", "Vypnout obrazovku", "Apaga la pantalla", "关闭屏幕", "स्क्रीन बंद", "إطفاء الشاشة", "স্ক্রিন বন্ধ", "اسکرین بند", "Matikan layar", "Zima skrini", "画面オフ", "ਸਕ੍ਰੀਨ ਬੰਦ", "स्क्रीन बंद", "స్క్రీన్ ఆఫ్", "Tắt màn hình", "Pateni layar", "Matikan skrin", "I-off ang screen", "خاموش کردن صفحه", "화면 끄기", "திரையை அணை", "Kashe allo", "સ્ક્રીન બંધ", "ปิดหน้าจอ", "ಪರದೆ ಆಫ್", "سکرین بند کړئ", "ማያ አጥፋ", "", "Pa ìbòjú", "ဖန်သားပြင်ပိတ်", "", "Вимкнути екран", "", "സ്ക്രീൻ ഓഫ്", "", "Ekranni o‘chirish", "", "स्क्रिन बन्द", "Gbanyụọ ihuenyo", "", "Ekranê bigire", "", "Vala isikrini", "", "Xidh shaashadda", "Cima isikrini", "Skerm af", "තිරය ක්‍රියාවිරහිත කරන්න", "បិទអេក្រង់", "Экранды өшіру", "Απενεργοποίηση οθόνης", ""},
	"launch":       {"Launch at start-up", "Beim Start ausführen", "Başlangıçta çalıştır", "Başlanğıcda işə sal", "Iniciar con Windows", "Lancer au démarrage", "Uruchom przy starcie", "Запускать при старте", "Iniciar com o Windows", "Avvia all'avvio", "Starten bij opstarten", "Pornește la startup", "Indítás a rendszerrel", "Starta vid uppstart", "Spustit při startu", "Inicia en arrencar", "开机启动", "स्टार्ट-अप पर चलाएँ", "التشغيل عند بدء النظام", "স্টার্ট-আপে চালু করুন", "اسٹارٹ اپ پر چلائیں", "Jalankan saat memulai", "Anza wakati wa kuwasha", "起動時に実行", "ਸ਼ੁਰੂਆਤ ਤੇ ਚਲਾਓ", "सुरू होताना चालवा", "ప్రారంభంలో అమలు చేయి", "Chạy khi khởi động", "Mlaku pas wiwitan", "Mulakan semasa permulaan", "Patakbuhin sa pagsisimula", "اجرا هنگام راه‌اندازی", "시작 시 실행", "தொடக்கத்தில் இயக்கு", "Fara a farawa", "શરૂ થતી વખતે ચલાવો", "เปิดเมื่อเริ่มระบบ", "ಪ್ರಾರಂಭದಲ್ಲಿ ಚಲಾಯಿಸಿ", "د پیل پر مهال چل کړئ", "ሲጀምር አሂድ", "", "Bẹ̀rẹ̀ nígbà ìbẹ̀rẹ̀", "စတင်ချိန်တွင်ဖွင့်", "", "Запускати під час старту", "", "ആരംഭത്തിൽ പ്രവർത്തിപ്പിക്കുക", "", "Ishga tushganda boshlash", "", "सुरु हुँदा चलाउनुहोस्", "Malite mgbe e bidoro", "", "Di destpêkê de bimeşîne", "", "Qala lapho uqala", "", "Ku bilow bilowga", "Qala xa kuqala", "Begin met opstart", "ආරම්භයේදී ධාවනය කරන්න", "ដំណើរការពេលចាប់ផ្តើម", "Іске қосылғанда бастау", "Εκκίνηση με τα Windows", "Launch at startup"},
	"mic":          {"Mic", "Mikro", "Mikrofon", "Mikrofon", "Micro", "Micro", "Mikrofon", "Микрофон", "Mic", "Mic", "Microfoon", "Microfon", "Mikrofon", "Mikrofon", "Mikrofon", "Micròfon", "麦克风", "माइक", "الميكروفون", "মাইক", "مائیک", "Mikrofon", "Maikrofoni", "マイク", "ਮਾਈਕ", "माइक", "మైక్", "Micro", "Mikropon", "Mikrofon", "Mic", "میکروفون", "마이크", "மைக்", "Makirufo", "માઇક", "ไมค์", "ಮೈಕ್", "مایکروفون", "ማይክሮፎን", "", "Gbohùngbohùn", "မိုက်", "", "Мікрофон", "", "മൈക്ക്", "", "Mikrofon", "", "माइक", "Maịkrọfon", "", "Mîkrofon", "", "Imakrofoni", "", "Makarafoon", "Imakrofoni", "Mikrofoon", "මයික්‍රොෆෝනය", "មីក្រូហ្វូន", "Микрофон", "Μικρόφωνο", ""},
	"lock":         {"Lock", "Sperren", "Kilitle", "Kilidlə", "Bloquear", "Verrouiller", "Zablokuj", "Блокировка", "Bloquear", "Blocca", "Vergrendelen", "Blocare", "Zárolás", "Lås", "Zamknout", "Bloqueja", "锁定", "लॉक", "قفل", "লক", "لاک", "Kunci", "Funga", "ロック", "ਲਾਕ", "लॉक", "లాక్", "Khóa", "Kunci", "Kunci", "I-lock", "قفل", "잠금", "பூட்டு", "Kulle", "લૉક", "ล็อก", "ಲಾಕ್", "قفل", "ቆልፍ", "", "Tì", "လော့ခ်ချ", "", "Заблокувати", "", "ലോക്ക്", "", "Qulflash", "", "लक", "Mkpọchi", "", "Qifil bike", "", "Khiya", "", "Quful", "Khiya", "Sluit", "අගුළු දමන්න", "ចាក់សោ", "Құлыптау", "Κλείδωμα", ""},
	"languages":    {"Languages", "Sprachen", "Diller", "Dillər", "Idiomas", "Langues", "Języki", "Языки", "Idiomas", "Lingue", "Talen", "Limbi", "Nyelvek", "Språk", "Jazyky", "Idiomes", "语言", "भाषाएँ", "اللغات", "ভাষা", "زبانیں", "Bahasa", "Lugha", "言語", "ਭਾਸ਼ਾਵਾਂ", "भाषा", "భాషలు", "Ngôn ngữ", "Basa", "Bahasa", "Mga wika", "زبان‌ها", "언어", "மொழிகள்", "Harsuna", "ભાષાઓ", "ภาษา", "ಭಾಷೆಗಳು", "ژبې", "ቋንቋዎች", "", "Àwọn èdè", "ဘာသာစကားများ", "", "Мови", "", "ഭാഷകൾ", "", "Tillar", "", "भाषाहरू", "Asụsụ", "", "Ziman", "", "Izilimi", "", "Luqado", "Iilwimi", "Tale", "භාෂා", "ភាសា", "Тілдер", "Γλώσσες", ""},
	"back":         {"Back", "Zurück", "Geri", "Geri", "Atrás", "Retour", "Wstecz", "Назад", "Voltar", "Indietro", "Terug", "Înapoi", "Vissza", "Tillbaka", "Zpět", "Enrere", "返回", "वापस", "رجوع", "ফিরে যান", "واپس", "Kembali", "Rudi", "戻る", "ਪਿੱਛੇ", "मागे", "వెనుకకు", "Quay lại", "Bali", "Kembali", "Bumalik", "بازگشت", "뒤로", "பின்செல்", "Koma", "પાછળ", "กลับ", "ಹಿಂದೆ", "شاته", "ተመለስ", "", "Padà", "နောက်သို့", "", "Назад", "", "പിന്നോട്ട്", "", "Orqaga", "", "पछाडि", "Laghachi", "", "Vegere", "", "Emuva", "", "Dib u noqo", "Emva", "Terug", "ආපසු", "ត្រឡប់ក្រោយ", "Артқа", "Πίσω", ""},
	"builtin":      {"Built-in display", "Integriertes Display", "Dahili ekran", "Daxili ekran", "Pantalla integrada", "Écran intégré", "Wbudowany ekran", "Встроенный экран", "Tela integrada", "Schermo integrato", "Ingebouwd scherm", "Ecran integrat", "Beépített kijelző", "Inbyggd skärm", "Vestavěný displej", "Pantalla integrada", "内置显示器", "इनबिल्ट डिस्प्ले", "الشاشة المدمجة", "বিল্ট-ইন ডিসপ্লে", "بلٹ اِن ڈسپلے", "Layar bawaan", "Skrini ya ndani", "内蔵ディスプレイ", "ਇਨਬਿਲਟ ਡਿਸਪਲੇ", "इनबिल्ट डिस्प्ले", "అంతర్నిర్మిత డిస్‌ప్లే", "Màn hình tích hợp", "Layar internal", "Paparan terbina dalam", "Built-in na display", "نمایشگر داخلی", "내장 디스플레이", "உள்ளமைந்த காட்சி", "Allon da aka gina", "ઇનબિલ્ટ ડિસ્પ્લે", "จอภาพในตัว", "ಅಂತರ್ನಿರ್ಮಿತ ಪ್ರದರ್ಶನ", "ایښودل شوی سکرین", "አብሮ የተሰራ ማያ", "", "Ìbòjú inú", "ပါဝင်သောဖန်သားပြင်", "", "Вбудований екран", "", "ഇൻബിൽറ്റ് ഡിസ്പ്ലേ", "", "O‘rnatilgan ekran", "", "इनबिल्ट डिस्प्ले", "Ihuenyo e wuru ewu", "", "Ekrana çêkirî", "", "Isikrini esakhelwe ngaphakathi", "", "Shaashad gudaha ah", "Isikrini esakhelwe ngaphakathi", "Ingeboude skerm", "ඇතුළත් තිරය", "អេក្រង់ភ្ជាប់មកស្រាប់", "Кіріктірілген экран", "Ενσωματωμένη οθόνη", ""},
	"looking":      {"Looking for displays…", "Displays werden gesucht…", "Ekranlar aranıyor…", "Ekranlar axtarılır…", "Buscando pantallas…", "Recherche des écrans…", "Szukanie ekranów…", "Поиск мониторов…", "Procurando monitores…", "Ricerca degli schermi…", "Schermen zoeken…", "Se caută ecrane…", "Kijelzők keresése…", "Söker efter skärmar…", "Hledání displejů…", "S'estan cercant pantalles…", "正在查找显示器…", "डिस्प्ले खोजे जा रहे हैं…", "جارٍ البحث عن الشاشات…", "ডিসপ্লে খোঁজা হচ্ছে…", "ڈسپلے تلاش کیے جا رہے ہیں…", "Mencari layar…", "Inatafuta skrini…", "ディスプレイを検索中…", "ਡਿਸਪਲੇ ਲੱਭੇ ਜਾ ਰਹੇ ਹਨ…", "डिस्प्ले शोधत आहे…", "డిస్‌ప్లేల కోసం వెతుకుతోంది…", "Đang tìm màn hình…", "Nggoleki layar…", "Mencari paparan…", "Naghahanap ng mga display…", "در حال جستجوی نمایشگرها…", "디스플레이 찾는 중…", "காட்சிகளைத் தேடுகிறது…", "Ana neman allo…", "ડિસ્પ્લે શોધી રહ્યા છીએ…", "กำลังค้นหาจอภาพ…", "ಪ್ರದರ್ಶನಗಳನ್ನು ಹುಡುಕಲಾಗುತ್ತಿದೆ…", "د سکرینونو لټون…", "ማያዎችን በመፈለግ ላይ…", "", "Ń wá àwọn ìbòjú…", "ဖန်သားပြင်ရှာနေသည်…", "", "Пошук екранів…", "", "ഡിസ്പ്ലേകൾ തിരയുന്നു…", "", "Ekranlar qidirilmoqda…", "", "डिस्प्लेहरू खोज्दै…", "Na-achọ ihuenyo…", "", "Li ekranan digere…", "", "Iyatesha izikrini…", "", "Raadinta shaashadaha…", "Ukukhangela izikrini…", "Soek na skerms…", "තිර සොයමින්…", "កំពុងស្វែងរកអេក្រង់…", "Экрандар ізделуде…", "Αναζήτηση οθονών…", ""},
	"nodisp":       {"No adjustable displays found. Switch on DDC/CI in the monitor's own menu.", "Keine einstellbaren Displays gefunden. Aktivieren Sie DDC/CI im Menü des Monitors.", "Ayarlanabilir ekran bulunamadı. Monitörün kendi menüsünden DDC/CI'yi açın.", "Tənzimlənə bilən ekran tapılmadı. Monitorun öz menyusundan DDC/CI-ni aktiv edin.", "No se encontraron pantallas ajustables. Activa DDC/CI en el menú del monitor.", "Aucun écran réglable trouvé. Activez DDC/CI dans le menu du moniteur.", "Nie znaleziono regulowanych ekranów. Włącz DDC/CI w menu monitora.", "Регулируемые мониторы не найдены. Включите DDC/CI в меню монитора.", "Nenhum monitor ajustável encontrado. Ative o DDC/CI no menu do monitor.", "Nessuno schermo regolabile trovato. Attiva DDC/CI nel menu del monitor.", "", "", "", "", "", "", "未找到可调节的显示器。请在显示器菜单中开启 DDC/CI。", "समायोज्य डिस्प्ले नहीं मिला। मॉनिटर के अपने मेनू में DDC/CI चालू करें।", "لم يتم العثور على شاشات قابلة للضبط. فعّل DDC/CI من قائمة الشاشة.", "সামঞ্জস্যযোগ্য ডিসপ্লে পাওয়া যায়নি। মনিটরের মেনুতে DDC/CI চালু করুন।", "قابلِ ایڈجسٹ ڈسپلے نہیں ملا۔ مانیٹر کے مینو میں DDC/CI آن کریں۔", "Layar yang dapat diatur tidak ditemukan. Aktifkan DDC/CI di menu monitor.", "Hakuna skrini inayoweza kurekebishwa iliyopatikana. Washa DDC/CI kwenye menyu ya kifuatilizi.", "調整可能なディスプレイが見つかりません。モニターのメニューでDDC/CIを有効にしてください。", "ਐਡਜਸਟ ਕਰਨ ਯੋਗ ਡਿਸਪਲੇ ਨਹੀਂ ਮਿਲਿਆ। ਮਾਨੀਟਰ ਦੇ ਮੀਨੂ ਵਿੱਚ DDC/CI ਚਾਲੂ ਕਰੋ।", "समायोजित करता येणारा डिस्प्ले सापडला नाही. मॉनिटरच्या मेनूमध्ये DDC/CI चालू करा.", "సర్దుబాటు చేయగల డిస్‌ప్లే కనుగొనబడలేదు. మానిటర్ మెనూలో DDC/CIని ఆన్ చేయండి.", "Không tìm thấy màn hình có thể điều chỉnh. Hãy bật DDC/CI trong menu màn hình.", "Ora ana layar sing bisa diatur. Aktifake DDC/CI ing menu monitor.", "", "Walang nahanap na maaaring i-adjust na display. I-on ang DDC/CI sa menu ng monitor.", "نمایشگر قابل تنظیمی پیدا نشد. DDC/CI را در منوی مانیتور روشن کنید.", "조절 가능한 디스플레이를 찾을 수 없습니다. 모니터 메뉴에서 DDC/CI를 켜세요.", "சரிசெய்யக்கூடிய காட்சி எதுவும் இல்லை. மானிட்டர் மெனுவில் DDC/CI ஐ இயக்கவும்.", "Ba a sami allon da za a iya daidaita ba. Kunna DDC/CI a menu na na'urar duba.", "સમાયોજિત કરી શકાય તેવું ડિસ્પ્લે મળ્યું નથી. મોનિટરના મેનૂમાં DDC/CI ચાલુ કરો.", "ไม่พบจอภาพที่ปรับได้ โปรดเปิด DDC/CI ในเมนูของจอภาพ", "ಹೊಂದಿಸಬಹುದಾದ ಪ್ರದರ್ಶನ ಕಂಡುಬಂದಿಲ್ಲ. ಮಾನಿಟರ್ ಮೆನುವಿನಲ್ಲಿ DDC/CI ಆನ್ ಮಾಡಿ.", "", "", "", "", "ချိန်ညှိနိုင်သော ဖန်သားပြင် မတွေ့ပါ။ မော်နီတာမီနူးတွင် DDC/CI ဖွင့်ပါ။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"helptitle":    {"How Lumistar works", "So funktioniert Lumistar", "Lumistar nasıl çalışır", "Lumistar necə işləyir", "Cómo funciona Lumistar", "Comment fonctionne Lumistar", "Jak działa Lumistar", "Как работает Lumistar", "Como o Lumistar funciona", "Come funziona Lumistar", "Zo werkt Lumistar", "Cum funcționează Lumistar", "Így működik a Lumistar", "Så fungerar Lumistar", "Jak Lumistar funguje", "Com funciona Lumistar", "Lumistar 的工作方式", "Lumistar कैसे काम करता है", "كيف يعمل Lumistar", "Lumistar কীভাবে কাজ করে", "Lumistar کیسے کام کرتا ہے", "Cara kerja Lumistar", "Jinsi Lumistar inavyofanya kazi", "Lumistarの使い方", "Lumistar ਕਿਵੇਂ ਕੰਮ ਕਰਦਾ ਹੈ", "Lumistar कसे काम करते", "Lumistar ఎలా పనిచేస్తుంది", "Cách Lumistar hoạt động", "Cara kerja Lumistar", "", "Paano gumagana ang Lumistar", "Lumistar چگونه کار می‌کند", "Lumistar 사용 방법", "Lumistar எவ்வாறு செயல்படுகிறது", "Yadda Lumistar ke aiki", "Lumistar કેવી રીતે કામ કરે છે", "Lumistar ทำงานอย่างไร", "Lumistar ಹೇಗೆ ಕೆಲಸ ಮಾಡುತ್ತದೆ", "", "", "", "", "Lumistar အလုပ်လုပ်ပုံ", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"intro":        {"Lumistar controls your screen brightness and volume from the system tray. Click its icon to open or close this panel.", "Lumistar steuert Helligkeit und Lautstärke direkt aus dem Infobereich. Klicken Sie auf das Symbol, um dieses Panel zu öffnen oder zu schließen.", "Lumistar, ekran parlaklığını ve sesi sistem tepsisinden kontrol eder. Bu paneli açmak veya kapatmak için simgesine tıklayın.", "Lumistar ekran parlaqlığını və səsi sistem tepsisindən idarə edir. Paneli açmaq və ya bağlamaq üçün işarəsinə klikləyin.", "Lumistar controla el brillo y el volumen desde la bandeja del sistema. Haz clic en su icono para abrir o cerrar este panel.", "Lumistar règle la luminosité et le volume depuis la zone de notification. Cliquez sur son icône pour ouvrir ou fermer ce panneau.", "Lumistar steruje jasnością ekranu i głośnością z obszaru powiadomień. Kliknij jego ikonę, aby otworzyć lub zamknąć ten panel.", "Lumistar управляет яркостью экрана и громкостью из системного трея. Щёлкните его значок, чтобы открыть или закрыть панель.", "O Lumistar controla o brilho da tela e o volume pela bandeja do sistema. Clique no ícone para abrir ou fechar este painel.", "Lumistar controlla luminosità e volume dall'area di notifica. Fai clic sulla sua icona per aprire o chiudere questo pannello.", "Lumistar regelt de helderheid en het volume van je scherm vanuit het systeemvak. Klik op het pictogram om dit paneel te openen of te sluiten.", "Lumistar controlează luminozitatea ecranului și volumul din zona de notificare. Faceți clic pe pictogramă pentru a deschide sau închide acest panou.", "A Lumistar a tálcáról szabályozza a képernyő fényerejét és a hangerőt. Kattintson az ikonjára a panel megnyitásához vagy bezárásához.", "Lumistar styr skärmens ljusstyrka och volymen från aktivitetsfältet. Klicka på ikonen för att öppna eller stänga panelen.", "Lumistar ovládá jas obrazovky a hlasitost z oznamovací oblasti. Kliknutím na ikonu panel otevřete nebo zavřete.", "Lumistar controla la brillantor de la pantalla i el volum des de la safata del sistema. Fes clic a la seva icona per obrir o tancar aquest tauler.", "Lumistar 可在系统托盘中控制屏幕亮度和音量。点击其图标即可打开或关闭此面板。", "Lumistar सिस्टम ट्रे से स्क्रीन की चमक और आवाज़ को नियंत्रित करता है। इस पैनल को खोलने या बंद करने के लिए उसके आइकन पर क्लिक करें।", "يتحكم Lumistar في سطوع الشاشة ومستوى الصوت من علبة النظام. انقر على أيقونته لفتح هذه اللوحة أو إغلاقها.", "Lumistar সিস্টেম ট্রে থেকে স্ক্রিনের উজ্জ্বলতা ও ভলিউম নিয়ন্ত্রণ করে। এই প্যানেল খুলতে বা বন্ধ করতে এর আইকনে ক্লিক করুন।", "Lumistar سسٹم ٹرے سے اسکرین کی روشنی اور آواز کنٹرول کرتا ہے۔ اس پینل کو کھولنے یا بند کرنے کے لیے اس کے آئیکن پر کلک کریں۔", "Lumistar mengatur kecerahan layar dan volume dari baki sistem. Klik ikonnya untuk membuka atau menutup panel ini.", "Lumistar hudhibiti mwangaza wa skrini na sauti kutoka kwenye trei ya mfumo. Bofya aikoni yake kufungua au kufunga paneli hii.", "Lumistarはシステムトレイから画面の明るさと音量を操作します。アイコンをクリックするとこのパネルを開閉できます。", "Lumistar ਸਿਸਟਮ ਟ੍ਰੇ ਤੋਂ ਸਕ੍ਰੀਨ ਦੀ ਚਮਕ ਅਤੇ ਆਵਾਜ਼ ਨੂੰ ਕੰਟਰੋਲ ਕਰਦਾ ਹੈ। ਇਸ ਪੈਨਲ ਨੂੰ ਖੋਲ੍ਹਣ ਜਾਂ ਬੰਦ ਕਰਨ ਲਈ ਇਸਦੇ ਆਈਕਨ 'ਤੇ ਕਲਿੱਕ ਕਰੋ।", "Lumistar सिस्टम ट्रेमधून स्क्रीनची चमक आणि आवाज नियंत्रित करते. हा पॅनेल उघडण्यासाठी किंवा बंद करण्यासाठी त्याच्या आयकॉनवर क्लिक करा.", "Lumistar సిస్టమ్ ట్రే నుండి స్క్రీన్ ప్రకాశం మరియు వాల్యూమ్‌ను నియంత్రిస్తుంది. ఈ ప్యానెల్‌ను తెరవడానికి లేదా మూసివేయడానికి దాని ఐకాన్‌ను క్లిక్ చేయండి.", "Lumistar điều khiển độ sáng màn hình và âm lượng từ khay hệ thống. Nhấp vào biểu tượng để mở hoặc đóng bảng này.", "Lumistar ngatur padhange layar lan volume saka baki sistem. Klik ikoné kanggo mbukak utawa nutup panel iki.", "", "Kinokontrol ng Lumistar ang liwanag ng screen at lakas ng tunog mula sa system tray. I-click ang icon nito para buksan o isara ang panel na ito.", "Lumistar روشنایی صفحه و میزان صدا را از سینی سیستم کنترل می‌کند. برای باز یا بسته کردن این پنل روی نمادش کلیک کنید.", "Lumistar는 시스템 트레이에서 화면 밝기와 볼륨을 제어합니다. 아이콘을 클릭하면 이 패널이 열리고 닫힙니다.", "Lumistar சிஸ்டம் ட்ரேயிலிருந்து திரை பிரகாசத்தையும் ஒலியளவையும் கட்டுப்படுத்துகிறது. இந்த பேனலைத் திறக்க அல்லது மூட அதன் ஐகானைக் கிளிக் செய்யவும்.", "Lumistar yana sarrafa hasken allo da ƙarar sauti daga tiren tsarin. Danna alamarsa don buɗe ko rufe wannan allon.", "Lumistar સિસ્ટમ ટ્રેમાંથી સ્ક્રીનની તેજ અને વોલ્યુમ નિયંત્રિત કરે છે. આ પેનલ ખોલવા અથવા બંધ કરવા તેના આઇકન પર ક્લિક કરો.", "Lumistar ควบคุมความสว่างหน้าจอและระดับเสียงจากซิสเต็มเทรย์ คลิกไอคอนเพื่อเปิดหรือปิดแผงนี้", "Lumistar ಸಿಸ್ಟಂ ಟ್ರೇಯಿಂದ ಪರದೆಯ ಹೊಳಪು ಮತ್ತು ವಾಲ್ಯೂಮ್ ನಿಯಂತ್ರಿಸುತ್ತದೆ. ಈ ಪ್ಯಾನೆಲ್ ತೆರೆಯಲು ಅಥವಾ ಮುಚ್ಚಲು ಅದರ ಐಕಾನ್ ಕ್ಲಿಕ್ ಮಾಡಿ.", "", "", "", "", "Lumistar သည် system tray မှ မျက်နှာပြင်အလင်းနှင့် အသံကို ထိန်းချုပ်ပေးသည်။ ဤပန်နယ်ကို ဖွင့်ရန် သို့မဟုတ် ပိတ်ရန် ၎င်း၏ အိုင်ကွန်ကို နှိပ်ပါ။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_brightness": {"One slider per display. The 10 to 100 buttons set every display at once. External monitors need DDC/CI switched on.", "Ein Regler pro Display. Die Tasten 10 bis 100 stellen alle Displays gleichzeitig ein. Externe Monitore benötigen aktiviertes DDC/CI.", "Her ekran için bir kaydırıcı. 10–100 düğmeleri tüm ekranları aynı anda ayarlar. Harici monitörlerde DDC/CI açık olmalıdır.", "Hər ekran üçün bir sürüşdürücü. 10–100 düymələri bütün ekranları eyni anda tənzimləyir. Xarici monitorlarda DDC/CI aktiv olmalıdır.", "Un control deslizante por pantalla. Los botones de 10 a 100 ajustan todas las pantallas a la vez. Los monitores externos necesitan DDC/CI activado.", "Un curseur par écran. Les boutons 10 à 100 règlent tous les écrans à la fois. Les moniteurs externes nécessitent DDC/CI activé.", "Jeden suwak na ekran. Przyciski od 10 do 100 ustawiają wszystkie ekrany naraz. Monitory zewnętrzne wymagają włączonego DDC/CI.", "Один ползунок на каждый экран. Кнопки от 10 до 100 задают яркость всех экранов сразу. Для внешних мониторов должен быть включён DDC/CI.", "Um controle deslizante por monitor. Os botões de 10 a 100 ajustam todos os monitores de uma vez. Monitores externos precisam do DDC/CI ativado.", "Un cursore per ogni schermo. I pulsanti da 10 a 100 regolano tutti gli schermi insieme. I monitor esterni richiedono DDC/CI attivo.", "Eén schuifregelaar per scherm. De knoppen 10 tot 100 stellen alle schermen tegelijk in. Externe monitors hebben DDC/CI nodig.", "Un cursor pentru fiecare ecran. Butoanele de la 10 la 100 setează toate ecranele odată. Monitoarele externe necesită DDC/CI activat.", "Kijelzőnként egy csúszka. A 10–100 gombok az összes kijelzőt egyszerre állítják. A külső monitorokon be kell kapcsolni a DDC/CI-t.", "Ett reglage per skärm. Knapparna 10 till 100 ställer in alla skärmar samtidigt. Externa bildskärmar kräver att DDC/CI är på.", "Jeden posuvník na displej. Tlačítka 10 až 100 nastaví všechny displeje najednou. Externí monitory potřebují zapnuté DDC/CI.", "Un control lliscant per pantalla. Els botons del 10 al 100 ajusten totes les pantalles alhora. Els monitors externs necessiten DDC/CI activat.", "每个显示器一个滑块。10 到 100 的按钮可一次设置所有显示器。外接显示器需开启 DDC/CI。", "हर डिस्प्ले के लिए एक स्लाइडर। 10 से 100 के बटन सभी डिस्प्ले को एक साथ सेट करते हैं। बाहरी मॉनिटर में DDC/CI चालू होना चाहिए।", "شريط تمرير لكل شاشة. أزرار 10 إلى 100 تضبط كل الشاشات دفعة واحدة. تحتاج الشاشات الخارجية إلى تفعيل DDC/CI.", "প্রতিটি ডিসপ্লের জন্য একটি স্লাইডার। ১০ থেকে ১০০ বোতাম সব ডিসপ্লে একসাথে সেট করে। বাহ্যিক মনিটরে DDC/CI চালু থাকতে হবে।", "ہر ڈسپلے کے لیے ایک سلائیڈر۔ 10 سے 100 کے بٹن تمام ڈسپلے ایک ساتھ سیٹ کرتے ہیں۔ بیرونی مانیٹر میں DDC/CI آن ہونا چاہیے۔", "Satu penggeser per layar. Tombol 10 sampai 100 mengatur semua layar sekaligus. Monitor eksternal memerlukan DDC/CI aktif.", "Kitelezi kimoja kwa kila skrini. Vitufe vya 10 hadi 100 huweka skrini zote kwa pamoja. Vifuatilizi vya nje vinahitaji DDC/CI iwashwe.", "ディスプレイごとに1つのスライダー。10〜100のボタンで全ディスプレイを一度に設定します。外付けモニターはDDC/CIを有効にしてください。", "ਹਰ ਡਿਸਪਲੇ ਲਈ ਇੱਕ ਸਲਾਈਡਰ। 10 ਤੋਂ 100 ਬਟਨ ਸਾਰੇ ਡਿਸਪਲੇ ਇੱਕੋ ਵਾਰ ਸੈੱਟ ਕਰਦੇ ਹਨ। ਬਾਹਰੀ ਮਾਨੀਟਰ ਵਿੱਚ DDC/CI ਚਾਲੂ ਹੋਣਾ ਚਾਹੀਦਾ ਹੈ।", "प्रत्येक डिस्प्लेसाठी एक स्लायडर. 10 ते 100 बटणे सर्व डिस्प्ले एकाच वेळी सेट करतात. बाह्य मॉनिटरमध्ये DDC/CI चालू असणे आवश्यक आहे.", "ప్రతి డిస్‌ప్లేకు ఒక స్లైడర్. 10 నుండి 100 బటన్లు అన్ని డిస్‌ప్లేలను ఒకేసారి సెట్ చేస్తాయి. బాహ్య మానిటర్లలో DDC/CI ఆన్‌లో ఉండాలి.", "Mỗi màn hình một thanh trượt. Các nút từ 10 đến 100 đặt tất cả màn hình cùng lúc. Màn hình ngoài cần bật DDC/CI.", "Siji panggeser kanggo saben layar. Tombol 10 nganti 100 ngatur kabeh layar bebarengan. Monitor njaba kudu ngaktifake DDC/CI.", "", "Isang slider para sa bawat display. Ang mga button na 10 hanggang 100 ay nagse-set ng lahat ng display nang sabay. Kailangang naka-on ang DDC/CI sa mga external monitor.", "برای هر نمایشگر یک لغزنده. دکمه‌های ۱۰ تا ۱۰۰ همه نمایشگرها را یکجا تنظیم می‌کنند. مانیتورهای خارجی باید DDC/CI روشن داشته باشند.", "디스플레이마다 슬라이더 하나. 10~100 버튼은 모든 디스플레이를 한 번에 설정합니다. 외부 모니터는 DDC/CI를 켜야 합니다.", "ஒவ்வொரு காட்சிக்கும் ஒரு ஸ்லைடர். 10 முதல் 100 பொத்தான்கள் அனைத்து காட்சிகளையும் ஒரே நேரத்தில் அமைக்கும். வெளிப்புற மானிட்டர்களில் DDC/CI இயக்கப்பட்டிருக்க வேண்டும்.", "Mai zamiya ɗaya ga kowane allo. Maɓallan 10 zuwa 100 suna saita dukkan allo a lokaci guda. Na'urorin duba na waje suna buƙatar DDC/CI a kunne.", "દરેક ડિસ્પ્લે માટે એક સ્લાઇડર. 10 થી 100 બટનો બધા ડિસ્પ્લે એકસાથે સેટ કરે છે. બાહ્ય મોનિટરમાં DDC/CI ચાલુ હોવું જોઈએ.", "สไลเดอร์หนึ่งตัวต่อจอภาพหนึ่งจอ ปุ่ม 10 ถึง 100 ตั้งค่าทุกจอพร้อมกัน จอภายนอกต้องเปิด DDC/CI", "ಪ್ರತಿ ಡಿಸ್‌ಪ್ಲೇಗೆ ಒಂದು ಸ್ಲೈಡರ್. 10 ರಿಂದ 100 ಬಟನ್‌ಗಳು ಎಲ್ಲಾ ಡಿಸ್‌ಪ್ಲೇಗಳನ್ನು ಒಟ್ಟಿಗೆ ಹೊಂದಿಸುತ್ತವೆ. ಹೊರಗಿನ ಮಾನಿಟರ್‌ಗಳಲ್ಲಿ DDC/CI ಆನ್ ಆಗಿರಬೇಕು.", "", "", "", "", "ဖန်သားပြင်တစ်ခုစီအတွက် slider တစ်ခု။ ၁၀ မှ ၁၀၀ ခလုတ်များသည် ဖန်သားပြင်အားလုံးကို တစ်ပြိုင်နက် သတ်မှတ်သည်။ ပြင်ပမော်နီတာများတွင် DDC/CI ဖွင့်ထားရမည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_volume":     {"Sets the master volume. The 10 to 100 buttons jump to that level.", "Stellt die Gesamtlautstärke ein. Die Tasten 10 bis 100 springen direkt auf diesen Wert.", "Ana sesi ayarlar. 10–100 düğmeleri doğrudan o seviyeye atlar.", "Əsas səsi tənzimləyir. 10–100 düymələri birbaşa həmin səviyyəyə keçir.", "Ajusta el volumen general. Los botones de 10 a 100 saltan a ese nivel.", "Règle le volume principal. Les boutons 10 à 100 passent directement à ce niveau.", "Ustawia głośność główną. Przyciski od 10 do 100 przeskakują do tego poziomu.", "Задаёт общую громкость. Кнопки от 10 до 100 сразу устанавливают этот уровень.", "Define o volume geral. Os botões de 10 a 100 vão direto para esse nível.", "Imposta il volume principale. I pulsanti da 10 a 100 passano subito a quel livello.", "Stelt het hoofdvolume in. De knoppen 10 tot 100 springen naar dat niveau.", "Setează volumul principal. Butoanele de la 10 la 100 sar direct la acel nivel.", "Beállítja a fő hangerőt. A 10–100 gombok közvetlenül erre a szintre ugranak.", "Ställer in huvudvolymen. Knapparna 10 till 100 hoppar direkt till den nivån.", "Nastaví hlavní hlasitost. Tlačítka 10 až 100 skočí přímo na danou úroveň.", "Ajusta el volum general. Els botons del 10 al 100 salten a aquest nivell.", "设置主音量。10 到 100 的按钮可直接跳到该音量。", "मुख्य आवाज़ सेट करता है। 10 से 100 के बटन सीधे उस स्तर पर जाते हैं।", "يضبط مستوى الصوت الرئيسي. أزرار 10 إلى 100 تنتقل مباشرة إلى هذا المستوى.", "মাস্টার ভলিউম সেট করে। ১০ থেকে ১০০ বোতাম সরাসরি সেই স্তরে যায়।", "مرکزی آواز سیٹ کرتا ہے۔ 10 سے 100 کے بٹن سیدھا اسی سطح پر جاتے ہیں۔", "Mengatur volume utama. Tombol 10 sampai 100 langsung ke level itu.", "Huweka sauti kuu. Vitufe vya 10 hadi 100 huruka moja kwa moja hadi kiwango hicho.", "マスター音量を設定します。10〜100のボタンでその音量にすぐ切り替わります。", "ਮੁੱਖ ਆਵਾਜ਼ ਸੈੱਟ ਕਰਦਾ ਹੈ। 10 ਤੋਂ 100 ਬਟਨ ਸਿੱਧੇ ਉਸ ਪੱਧਰ 'ਤੇ ਜਾਂਦੇ ਹਨ।", "मुख्य आवाज सेट करते. 10 ते 100 बटणे थेट त्या पातळीवर जातात.", "ప్రధాన వాల్యూమ్‌ను సెట్ చేస్తుంది. 10 నుండి 100 బటన్లు నేరుగా ఆ స్థాయికి వెళ్తాయి.", "Đặt âm lượng chính. Các nút từ 10 đến 100 chuyển ngay đến mức đó.", "Ngatur volume utama. Tombol 10 nganti 100 langsung menyang tingkat iku.", "", "Itinatakda ang master volume. Ang mga button na 10 hanggang 100 ay direktang lumilipat sa antas na iyon.", "صدای اصلی را تنظیم می‌کند. دکمه‌های ۱۰ تا ۱۰۰ مستقیم به همان سطح می‌روند.", "마스터 볼륨을 설정합니다. 10~100 버튼은 해당 값으로 바로 이동합니다.", "முதன்மை ஒலியளவை அமைக்கிறது. 10 முதல் 100 பொத்தான்கள் நேரடியாக அந்த நிலைக்குச் செல்லும்.", "Yana saita babbar ƙarar sauti. Maɓallan 10 zuwa 100 suna tsallakawa kai tsaye zuwa wannan matakin.", "મુખ્ય વોલ્યુમ સેટ કરે છે. 10 થી 100 બટનો સીધા તે સ્તર પર જાય છે.", "ตั้งระดับเสียงหลัก ปุ่ม 10 ถึง 100 จะข้ามไปยังระดับนั้นทันที", "ಮುಖ್ಯ ವಾಲ್ಯೂಮ್ ಹೊಂದಿಸುತ್ತದೆ. 10 ರಿಂದ 100 ಬಟನ್‌ಗಳು ನೇರವಾಗಿ ಆ ಮಟ್ಟಕ್ಕೆ ಹೋಗುತ್ತವೆ.", "", "", "", "", "အဓိကအသံကို သတ်မှတ်သည်။ ၁၀ မှ ၁၀၀ ခလုတ်များသည် ထိုအဆင့်သို့ တိုက်ရိုက်သွားသည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_mute":       {"Silences all sound and restores it. Your volume level is kept.", "Schaltet alle Töne stumm und wieder an. Die Lautstärke bleibt erhalten.", "Tüm sesi kapatır ve geri açar. Ses seviyeniz korunur.", "Bütün səsi söndürür və geri qaytarır. Səs səviyyəsi saxlanılır.", "Silencia todo el sonido y lo restaura. Se conserva el nivel de volumen.", "Coupe tout le son et le rétablit. Le niveau du volume est conservé.", "Wycisza cały dźwięk i go przywraca. Poziom głośności zostaje zachowany.", "Отключает весь звук и возвращает его. Уровень громкости сохраняется.", "Silencia todo o som e o restaura. O nível do volume é mantido.", "Silenzia tutti i suoni e li ripristina. Il livello del volume viene mantenuto.", "Dempt al het geluid en zet het weer aan. Je volumeniveau blijft behouden.", "Dezactivează tot sunetul și îl restabilește. Nivelul volumului se păstrează.", "Elnémítja az összes hangot, majd visszaállítja. A hangerőszint megmarad.", "Tystar allt ljud och återställer det. Din volymnivå behålls.", "Ztlumí veškerý zvuk a znovu ho zapne. Úroveň hlasitosti zůstane zachována.", "Silencia tot el so i el restaura. Es conserva el nivell de volum.", "将所有声音静音并可恢复。音量大小保持不变。", "सारी आवाज़ बंद करता है और फिर चालू करता है। आवाज़ का स्तर बना रहता है।", "يكتم كل الأصوات ويعيدها. يبقى مستوى الصوت محفوظاً.", "সব শব্দ বন্ধ করে ও আবার চালু করে। ভলিউমের স্তর অপরিবর্তিত থাকে।", "تمام آواز بند کرتا ہے اور دوبارہ چالو کرتا ہے۔ آواز کی سطح برقرار رہتی ہے۔", "Membisukan semua suara dan memulihkannya. Level volume tetap tersimpan.", "Hunyamazisha sauti zote na kuzirudisha. Kiwango cha sauti hubaki.", "すべての音を消音し、元に戻します。音量は保持されます。", "ਸਾਰੀ ਆਵਾਜ਼ ਬੰਦ ਕਰਦਾ ਹੈ ਅਤੇ ਮੁੜ ਚਾਲੂ ਕਰਦਾ ਹੈ। ਆਵਾਜ਼ ਦਾ ਪੱਧਰ ਬਣਿਆ ਰਹਿੰਦਾ ਹੈ।", "सर्व आवाज बंद करते आणि पुन्हा चालू करते. आवाजाची पातळी तशीच राहते.", "అన్ని శబ్దాలను మ్యూట్ చేసి తిరిగి ఆన్ చేస్తుంది. వాల్యూమ్ స్థాయి అలాగే ఉంటుంది.", "Tắt toàn bộ âm thanh và bật lại. Mức âm lượng được giữ nguyên.", "Mateni kabeh swara lan mbalekake maneh. Tingkat volume tetep disimpen.", "", "Pinapatay ang lahat ng tunog at ibinabalik ito. Nananatili ang antas ng volume.", "همه صداها را بی‌صدا می‌کند و دوباره برمی‌گرداند. سطح صدا حفظ می‌شود.", "모든 소리를 음소거하고 다시 켭니다. 볼륨 수준은 유지됩니다.", "எல்லா ஒலியையும் அடக்கி மீண்டும் இயக்குகிறது. ஒலியளவு நிலை அப்படியே இருக்கும்.", "Yana rufe duk sauti kuma yana dawo da shi. Matakin ƙara yana nan yadda yake.", "બધો અવાજ બંધ કરે છે અને ફરી ચાલુ કરે છે. વોલ્યુમ સ્તર જળવાઈ રહે છે.", "ปิดเสียงทั้งหมดและเปิดกลับ ระดับเสียงจะคงเดิม", "ಎಲ್ಲಾ ಧ್ವನಿಯನ್ನು ಮೌನಗೊಳಿಸಿ ಮತ್ತೆ ಚಾಲನೆ ಮಾಡುತ್ತದೆ. ವಾಲ್ಯೂಮ್ ಮಟ್ಟ ಉಳಿಯುತ್ತದೆ.", "", "", "", "", "အသံအားလုံးကို ပိတ်ပြီး ပြန်ဖွင့်ပေးသည်။ အသံအဆင့် မပြောင်းလဲပါ။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_night":      {"Dims every display to 20%. Press again to restore.", "Dimmt alle Displays auf 20 %. Erneut drücken stellt die Helligkeit wieder her.", "Tüm ekranları %20 parlaklığa düşürür. Tekrar basınca eski hâline döner.", "Bütün ekranları 20% parlaqlığa endirir. Yenidən basanda əvvəlki hala qayıdır.", "Reduce todas las pantallas al 20 %. Púlsalo de nuevo para restaurar.", "Réduit tous les écrans à 20 %. Appuyez à nouveau pour rétablir.", "Przyciemnia wszystkie ekrany do 20%. Naciśnij ponownie, aby przywrócić.", "Снижает яркость всех экранов до 20%. Повторное нажатие возвращает прежнюю.", "Reduz todos os monitores a 20%. Pressione de novo para restaurar.", "Riduce tutti gli schermi al 20%. Premi di nuovo per ripristinare.", "Dimt alle schermen tot 20%. Druk nogmaals om te herstellen.", "Reduce luminozitatea tuturor ecranelor la 20%. Apăsați din nou pentru a restabili.", "Minden kijelzőt 20%-ra halványít. Újabb megnyomásra visszaáll.", "Dämpar alla skärmar till 20 %. Tryck igen för att återställa.", "Ztlumí všechny displeje na 20 %. Dalším stisknutím obnovíte.", "Redueix totes les pantalles al 20 %. Prem de nou per restaurar.", "将所有显示器调暗至 20%。再按一次即可恢复。", "सभी डिस्प्ले को 20% पर मंद करता है। दोबारा दबाने पर पहले जैसा हो जाता है।", "يخفض سطوع كل الشاشات إلى 20%. اضغط مرة أخرى للاستعادة.", "সব ডিসপ্লে ২০% এ কমায়। আবার চাপলে আগের অবস্থায় ফেরে।", "تمام ڈسپلے کو 20% پر مدھم کرتا ہے۔ دوبارہ دبانے پر پہلے جیسا ہو جاتا ہے۔", "Meredupkan semua layar ke 20%. Tekan lagi untuk memulihkan.", "Hupunguza mwangaza wa skrini zote hadi 20%. Bonyeza tena kurejesha.", "すべてのディスプレイを20%に暗くします。もう一度押すと元に戻ります。", "ਸਾਰੇ ਡਿਸਪਲੇ 20% 'ਤੇ ਮੱਧਮ ਕਰਦਾ ਹੈ। ਦੁਬਾਰਾ ਦਬਾਉਣ 'ਤੇ ਪਹਿਲਾਂ ਵਾਂਗ ਹੋ ਜਾਂਦਾ ਹੈ।", "सर्व डिस्प्ले 20% वर मंद करते. पुन्हा दाबल्यावर पूर्ववत होते.", "అన్ని డిస్‌ప్లేలను 20% కి మసకబారుస్తుంది. మళ్లీ నొక్కితే పునరుద్ధరిస్తుంది.", "Giảm độ sáng mọi màn hình xuống 20%. Nhấn lại để khôi phục.", "Ngredupake kabeh layar dadi 20%. Pencet maneh kanggo mbalekake.", "", "Ginagawang 20% ang liwanag ng lahat ng display. Pindutin muli para ibalik.", "روشنایی همه نمایشگرها را به ۲۰٪ کاهش می‌دهد. با فشار دوباره بازمی‌گردد.", "모든 디스플레이를 20%로 어둡게 합니다. 다시 누르면 복원됩니다.", "அனைத்து காட்சிகளையும் 20% ஆக மங்கச் செய்கிறது. மீண்டும் அழுத்தினால் மீட்கும்.", "Yana rage hasken duk allo zuwa 20%. Sake dannawa yana dawo da shi.", "બધા ડિસ્પ્લેને 20% સુધી ઝાંખા કરે છે. ફરી દબાવતાં પાછા આવે છે.", "หรี่ทุกจอเหลือ 20% กดอีกครั้งเพื่อคืนค่า", "ಎಲ್ಲಾ ಡಿಸ್‌ಪ್ಲೇಗಳನ್ನು 20% ಗೆ ಮಂದಗೊಳಿಸುತ್ತದೆ. ಮತ್ತೆ ಒತ್ತಿದರೆ ಮರುಸ್ಥಾಪಿಸುತ್ತದೆ.", "", "", "", "", "ဖန်သားပြင်အားလုံးကို ၂၀% သို့ လျှော့ပေးသည်။ ထပ်နှိပ်လျှင် ပြန်ရောက်သည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_awake":      {"Stops your PC and screens from going to sleep.", "Verhindert, dass PC und Bildschirme in den Ruhezustand wechseln.", "Bilgisayarın ve ekranların uykuya geçmesini engeller.", "Kompüterin və ekranların yuxuya getməsinin qarşısını alır.", "Evita que el PC y las pantallas entren en suspensión.", "Empêche le PC et les écrans de se mettre en veille.", "Zapobiega usypianiu komputera i ekranów.", "Не даёт компьютеру и экранам уйти в спящий режим.", "Impede que o PC e as telas entrem em suspensão.", "Impedisce al PC e agli schermi di andare in sospensione.", "Voorkomt dat je pc en schermen in slaapstand gaan.", "Împiedică intrarea computerului și a ecranelor în repaus.", "Megakadályozza, hogy a számítógép és a képernyők alvó módba lépjenek.", "Hindrar datorn och skärmarna från att gå i viloläge.", "Zabrání přechodu počítače a obrazovek do režimu spánku.", "Evita que l'ordinador i les pantalles entrin en suspensió.", "防止电脑和屏幕进入睡眠。", "पीसी और स्क्रीन को स्लीप में जाने से रोकता है।", "يمنع الكمبيوتر والشاشات من الدخول في وضع السكون.", "পিসি ও স্ক্রিনকে ঘুমে যেতে দেয় না।", "پی سی اور اسکرین کو سلیپ میں جانے سے روکتا ہے۔", "Mencegah PC dan layar masuk ke mode tidur.", "Huzuia kompyuta na skrini kulala.", "PCと画面がスリープに入らないようにします。", "PC ਅਤੇ ਸਕ੍ਰੀਨ ਨੂੰ ਸਲੀਪ ਵਿੱਚ ਜਾਣ ਤੋਂ ਰੋਕਦਾ ਹੈ।", "पीसी आणि स्क्रीन स्लीपमध्ये जाण्यापासून रोखते.", "PC మరియు స్క్రీన్‌లు నిద్రలోకి వెళ్లకుండా ఆపుతుంది.", "Ngăn máy tính và màn hình chuyển sang chế độ ngủ.", "Nyegah PC lan layar mlebu mode turu.", "", "Pinipigilan ang PC at mga screen na mag-sleep.", "از رفتن رایانه و صفحه‌ها به حالت خواب جلوگیری می‌کند.", "PC와 화면이 절전 모드로 들어가지 않게 합니다.", "கணினியும் திரைகளும் உறக்கத்திற்குச் செல்லாமல் தடுக்கிறது.", "Yana hana kwamfuta da allo yin barci.", "PC અને સ્ક્રીનને સ્લીપમાં જતા અટકાવે છે.", "ป้องกันไม่ให้พีซีและหน้าจอเข้าสู่โหมดสลีป", "PC ಮತ್ತು ಪರದೆಗಳು ನಿದ್ರೆಗೆ ಹೋಗುವುದನ್ನು ತಡೆಯುತ್ತದೆ.", "", "", "", "", "ကွန်ပျူတာနှင့် ဖန်သားပြင်များ အိပ်မသွားစေရန် တားဆီးသည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_screenoff":  {"Turns your monitors off. Move the mouse to wake them.", "Schaltet die Monitore sofort aus. Maus bewegen, um sie zu wecken.", "Monitörleri kapatır. Uyandırmak için fareyi oynatın.", "Monitorları söndürür. Oyatmaq üçün siçanı tərpədin.", "Apaga los monitores. Mueve el ratón para reactivarlos.", "Éteint les moniteurs. Bougez la souris pour les réveiller.", "Wyłącza monitory. Poruszaj myszą, aby je obudzić.", "Выключает мониторы. Пошевелите мышью, чтобы включить.", "Desliga os monitores. Mova o mouse para ativá-los.", "Spegne i monitor. Muovi il mouse per riattivarli.", "Schakelt je monitors uit. Beweeg de muis om ze te wekken.", "Oprește monitoarele. Mișcați mouse-ul pentru a le reactiva.", "Kikapcsolja a monitorokat. Az egér megmozdításával ébreszthetők fel.", "Stänger av bildskärmarna. Rör på musen för att väcka dem.", "Vypne monitory. Pohybem myši je probudíte.", "Apaga els monitors. Mou el ratolí per reactivar-los.", "立即关闭显示器。移动鼠标即可唤醒。", "मॉनिटर तुरंत बंद करता है। जगाने के लिए माउस हिलाएँ।", "يطفئ الشاشات فوراً. حرّك الفأرة لإيقاظها.", "মনিটর বন্ধ করে। জাগাতে মাউস নাড়ান।", "مانیٹر فوراً بند کرتا ہے۔ جگانے کے لیے ماؤس ہلائیں۔", "Mematikan monitor. Gerakkan mouse untuk membangunkannya.", "Huzima vifuatilizi. Sogeza kipanya ili kuviamsha.", "モニターをすぐに消します。マウスを動かすと復帰します。", "ਮਾਨੀਟਰ ਤੁਰੰਤ ਬੰਦ ਕਰਦਾ ਹੈ। ਜਗਾਉਣ ਲਈ ਮਾਊਸ ਹਿਲਾਓ।", "मॉनिटर लगेच बंद करते. जागे करण्यासाठी माउस हलवा.", "మానిటర్లను వెంటనే ఆఫ్ చేస్తుంది. లేపడానికి మౌస్‌ను కదిలించండి.", "Tắt các màn hình ngay. Di chuyển chuột để đánh thức.", "Mateni monitor. Obahna mouse kanggo nangekake.", "", "Pinapatay ang mga monitor. Igalaw ang mouse para gisingin ang mga ito.", "مانیتورها را فوراً خاموش می‌کند. برای بیدار کردن ماوس را حرکت دهید.", "모니터를 바로 끕니다. 마우스를 움직이면 다시 켜집니다.", "மானிட்டர்களை உடனே அணைக்கிறது. எழுப்ப சுட்டியை நகர்த்தவும்.", "Yana kashe na'urorin duba. Motsa linzamin kwamfuta don tashe su.", "મોનિટર તરત બંધ કરે છે. જગાડવા માઉસ હલાવો.", "ปิดจอภาพทันที ขยับเมาส์เพื่อปลุก", "ಮಾನಿಟರ್‌ಗಳನ್ನು ತಕ್ಷಣ ಆಫ್ ಮಾಡುತ್ತದೆ. ಎಬ್ಬಿಸಲು ಮೌಸ್ ಅಲುಗಾಡಿಸಿ.", "", "", "", "", "မော်နီတာများကို ချက်ချင်းပိတ်သည်။ ပြန်နှိုးရန် မောက်စ်ကို လှုပ်ပါ။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_theme":      {"Switches between the dark and light themes.", "Wechselt zwischen dunklem und hellem Design.", "Koyu ve açık tema arasında geçiş yapar.", "Tünd və işıqlı tema arasında keçid edir.", "Cambia entre el tema oscuro y el claro.", "Bascule entre le thème sombre et le thème clair.", "Przełącza między ciemnym a jasnym motywem.", "Переключает тёмную и светлую темы.", "Alterna entre os temas escuro e claro.", "Passa dal tema scuro a quello chiaro.", "Wisselt tussen het donkere en lichte thema.", "Comută între tema întunecată și cea luminoasă.", "Váltás a sötét és a világos téma között.", "Växlar mellan mörkt och ljust tema.", "Přepíná mezi tmavým a světlým motivem.", "Canvia entre el tema fosc i el clar.", "在深色和浅色主题之间切换。", "डार्क और लाइट थीम के बीच बदलता है।", "يبدّل بين السمة الداكنة والفاتحة.", "ডার্ক ও লাইট থিমের মধ্যে পরিবর্তন করে।", "ڈارک اور لائٹ تھیم کے درمیان تبدیل کرتا ہے۔", "Beralih antara tema gelap dan terang.", "Hubadilisha kati ya mandhari meusi na meupe.", "ダークとライトのテーマを切り替えます。", "ਡਾਰਕ ਅਤੇ ਲਾਈਟ ਥੀਮ ਵਿਚਕਾਰ ਬਦਲਦਾ ਹੈ।", "डार्क आणि लाइट थीम दरम्यान बदलते.", "డార్క్ మరియు లైట్ థీమ్‌ల మధ్య మారుస్తుంది.", "Chuyển giữa giao diện tối và sáng.", "Ganti antarane tema peteng lan cerah.", "", "Lumilipat sa pagitan ng dark at light na tema.", "بین زمینه تیره و روشن جابه‌جا می‌شود.", "어두운 테마와 밝은 테마를 전환합니다.", "இருள் மற்றும் ஒளி தீம்களுக்கு இடையே மாற்றுகிறது.", "Yana sauya tsakanin duhu da haske.", "ડાર્ક અને લાઇટ થીમ વચ્ચે બદલે છે.", "สลับระหว่างธีมมืดและธีมสว่าง", "ಡಾರ್ಕ್ ಮತ್ತು ಲೈಟ್ ಥೀಮ್ ನಡುವೆ ಬದಲಾಯಿಸುತ್ತದೆ.", "", "", "", "", "အမှောင်နှင့် အလင်းအပြင်အဆင် အကြား ပြောင်းသည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_launch":     {"Starts Lumistar when you sign in to Windows.", "Startet Lumistar bei der Anmeldung bei Windows.", "Windows'ta oturum açtığınızda Lumistar'ı başlatır.", "Windows-a daxil olanda Lumistar-ı işə salır.", "Inicia Lumistar al iniciar sesión en Windows.", "Lance Lumistar à l'ouverture de session Windows.", "Uruchamia Lumistar po zalogowaniu do Windows.", "Запускает Lumistar при входе в Windows.", "Inicia o Lumistar quando você entra no Windows.", "Avvia Lumistar quando accedi a Windows.", "Start Lumistar wanneer je je aanmeldt bij Windows.", "Pornește Lumistar când vă conectați la Windows.", "Elindítja a Lumistart, amikor bejelentkezik a Windowsba.", "Startar Lumistar när du loggar in i Windows.", "Spustí Lumistar při přihlášení do Windows.", "Inicia Lumistar quan inicies la sessió a Windows.", "登录 Windows 时启动 Lumistar。", "Windows में साइन इन करने पर Lumistar शुरू करता है।", "يشغّل Lumistar عند تسجيل الدخول إلى Windows.", "Windows-এ সাইন ইন করলে Lumistar চালু করে।", "Windows میں سائن اِن کرنے پر Lumistar شروع کرتا ہے۔", "Menjalankan Lumistar saat Anda masuk ke Windows.", "Huanzisha Lumistar unapoingia kwenye Windows.", "Windowsにサインインしたときに Lumistar を起動します。", "Windows ਵਿੱਚ ਸਾਈਨ ਇਨ ਕਰਨ 'ਤੇ Lumistar ਸ਼ੁਰੂ ਕਰਦਾ ਹੈ।", "Windows मध्ये साइन इन केल्यावर Lumistar सुरू करते.", "Windows లో సైన్ ఇన్ చేసినప్పుడు Lumistar ను ప్రారంభిస్తుంది.", "Khởi chạy Lumistar khi bạn đăng nhập Windows.", "Miwiti Lumistar nalika sampeyan mlebu Windows.", "", "Sinisimulan ang Lumistar kapag nag-sign in ka sa Windows.", "هنگام ورود به ویندوز، Lumistar را اجرا می‌کند.", "Windows에 로그인할 때 Lumistar를 시작합니다.", "Windows இல் உள்நுழையும்போது Lumistar ஐத் தொடங்குகிறது.", "Yana fara Lumistar lokacin da ka shiga Windows.", "Windows માં સાઇન ઇન કરતાં Lumistar શરૂ કરે છે.", "เริ่ม Lumistar เมื่อคุณลงชื่อเข้าใช้ Windows", "Windows ಗೆ ಸೈನ್ ಇನ್ ಆದಾಗ Lumistar ಪ್ರಾರಂಭಿಸುತ್ತದೆ.", "", "", "", "", "Windows သို့ ဝင်သောအခါ Lumistar ကို စတင်ပေးသည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_mic":        {"Mutes or unmutes your microphone. A line crosses the icon while it is muted.", "Schaltet das Mikrofon stumm oder wieder ein. Bei Stummschaltung ist das Symbol durchgestrichen.", "Mikrofonu kapatır veya açar. Kapalıyken simgenin üstü çizilir.", "Mikrofonu söndürür və ya yandırır. Səssiz olanda işarənin üstündən xətt çəkilir.", "Silencia o activa el micrófono. El icono aparece tachado mientras está silenciado.", "Coupe ou réactive le micro. L'icône est barrée lorsqu'il est coupé.", "Wycisza lub włącza mikrofon. Gdy jest wyciszony, ikona jest przekreślona.", "Отключает или включает микрофон. Пока он отключён, значок перечёркнут.", "Silencia ou ativa o microfone. O ícone fica riscado enquanto está silenciado.", "Silenzia o riattiva il microfono. L'icona è barrata mentre è silenziato.", "Dempt je microfoon of zet hem weer aan. Het pictogram wordt rood en doorgestreept bij dempen.", "Dezactivează sau activează microfonul. Pictograma devine roșie și barată când este dezactivat.", "Elnémítja vagy visszakapcsolja a mikrofont. Némításkor az ikon piros és át van húzva.", "Tystar eller slår på mikrofonen. Ikonen blir röd och överstruken när den är tystad.", "Ztlumí nebo zapne mikrofon. Při ztlumení je ikona červená a přeškrtnutá.", "Silencia o activa el micròfon. La icona es torna vermella i ratllada quan està silenciat.", "将麦克风静音或取消静音。静音时图标上会出现斜线。", "माइक्रोफ़ोन को म्यूट या अनम्यूट करता है। म्यूट होने पर आइकन पर आड़ी रेखा दिखती है।", "يكتم الميكروفون أو يعيده. تظهر الأيقونة مشطوبة أثناء الكتم.", "মাইক্রোফোন মিউট বা আনমিউট করে। মিউট থাকলে আইকনে দাগ পড়ে।", "مائیک کو خاموش یا بحال کرتا ہے۔ خاموش ہونے پر آئیکن پر لکیر آ جاتی ہے۔", "Membisukan atau mengaktifkan mikrofon. Ikon dicoret saat dibisukan.", "Hunyamazisha au hurudisha maikrofoni. Aikoni huwa na mstari ikinyamazishwa.", "マイクをミュートまたは解除します。ミュート中はアイコンに斜線が入ります。", "ਮਾਈਕ੍ਰੋਫ਼ੋਨ ਨੂੰ ਮਿਊਟ ਜਾਂ ਅਨਮਿਊਟ ਕਰਦਾ ਹੈ। ਮਿਊਟ ਹੋਣ 'ਤੇ ਆਈਕਨ 'ਤੇ ਲਕੀਰ ਦਿਖਦੀ ਹੈ।", "मायक्रोफोन म्यूट किंवा अनम्यूट करते. म्यूट असताना आयकॉनवर रेघ दिसते.", "మైక్రోఫోన్‌ను మ్యూట్ లేదా అన్‌మ్యూట్ చేస్తుంది. మ్యూట్‌లో ఉన్నప్పుడు ఐకాన్‌పై గీత కనిపిస్తుంది.", "Tắt hoặc bật tiếng micro. Biểu tượng bị gạch chéo khi đang tắt tiếng.", "Mateni utawa nguripake mikropon. Ikon dicoret nalika dibisukan.", "", "Ini-mute o ina-unmute ang mikropono. May guhit ang icon kapag naka-mute.", "میکروفون را بی‌صدا یا فعال می‌کند. هنگام بی‌صدا بودن روی نماد خط می‌افتد.", "마이크를 음소거하거나 해제합니다. 음소거 중에는 아이콘에 사선이 표시됩니다.", "மைக்ரோஃபோனை ஒலியடக்கு அல்லது இயக்குகிறது. ஒலியடக்கப்பட்டிருக்கும்போது ஐகானில் கோடு விழும்.", "Yana rufe ko buɗe makirufo. Alamar tana da layi lokacin da aka rufe.", "માઇક્રોફોનને મ્યૂટ અથવા અનમ્યૂટ કરે છે. મ્યૂટ હોય ત્યારે આઇકન પર લીટી દેખાય છે.", "ปิดหรือเปิดเสียงไมโครโฟน ไอคอนจะมีเส้นขีดคร่อมเมื่อปิดเสียง", "ಮೈಕ್ರೊಫೋನ್ ಮ್ಯೂಟ್ ಅಥವಾ ಅನ್‌ಮ್ಯೂಟ್ ಮಾಡುತ್ತದೆ. ಮ್ಯೂಟ್ ಆಗಿದ್ದಾಗ ಐಕಾನ್ ಮೇಲೆ ಗೆರೆ ಕಾಣುತ್ತದೆ.", "", "", "", "", "မိုက်ခရိုဖုန်းကို အသံတိတ်ခြင်း သို့မဟုတ် ပြန်ဖွင့်ခြင်း ပြုလုပ်သည်။ အသံတိတ်နေစဉ် အိုင်ကွန်ပေါ်တွင် မျဉ်းတစ်ကြောင်း ပေါ်သည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_lock":       {"Locks your PC straight away, like pressing Win + L.", "Sperrt den PC sofort, wie mit Win + L.", "Bilgisayarı hemen kilitler, Win + L gibi.", "Kompüteri dərhal kilidləyir, Win + L kimi.", "Bloquea el PC al instante, como Win + L.", "Verrouille le PC immédiatement, comme Win + L.", "Natychmiast blokuje komputer, jak Win + L.", "Мгновенно блокирует компьютер, как Win + L.", "Bloqueia o PC na hora, como Win + L.", "Blocca subito il PC, come Win + L.", "Vergrendelt je pc direct, zoals Win + L.", "Blochează imediat computerul, ca la Win + L.", "Azonnal zárolja a számítógépet, mint a Win + L.", "Låser datorn direkt, som Win + L.", "Okamžitě zamkne počítač, stejně jako Win + L.", "Bloqueja l'ordinador a l'instant, com Win + L.", "立即锁定电脑，等同于 Win + L。", "पीसी को तुरंत लॉक करता है, Win + L की तरह।", "يقفل الكمبيوتر فوراً، مثل Win + L.", "পিসি সঙ্গে সঙ্গে লক করে, Win + L এর মতো।", "پی سی کو فوراً لاک کرتا ہے، Win + L کی طرح۔", "Langsung mengunci PC, seperti Win + L.", "Hufunga kompyuta mara moja, kama Win + L.", "PCをすぐにロックします。Win + L と同じです。", "PC ਨੂੰ ਤੁਰੰਤ ਲਾਕ ਕਰਦਾ ਹੈ, Win + L ਵਾਂਗ।", "पीसी लगेच लॉक करते, Win + L प्रमाणे.", "PC ని వెంటనే లాక్ చేస్తుంది, Win + L లాగా.", "Khóa máy ngay lập tức, giống Win + L.", "Langsung ngunci PC, kaya Win + L.", "", "Agad na nila-lock ang PC, tulad ng Win + L.", "رایانه را فوراً قفل می‌کند، مثل Win + L.", "Win + L처럼 PC를 즉시 잠급니다.", "கணினியை உடனே பூட்டுகிறது, Win + L போல.", "Yana kulle kwamfuta nan take, kamar Win + L.", "PC ને તરત લૉક કરે છે, Win + L ની જેમ.", "ล็อกพีซีทันที เหมือนกด Win + L", "PC ಅನ್ನು ತಕ್ಷಣ ಲಾಕ್ ಮಾಡುತ್ತದೆ, Win + L ನಂತೆ.", "", "", "", "", "ကွန်ပျူတာကို ချက်ချင်း လော့ခ်ချသည်၊ Win + L ကဲ့သို့။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_languages":  {"Opens the language list. Choose from 64 languages.", "Öffnet die Sprachliste. Wählen Sie aus 64 Sprachen.", "Dil listesini açar. 64 dil arasından seçin.", "Dil siyahısını açır. 64 dil arasından seçin.", "Abre la lista de idiomas. Elige entre 64 idiomas.", "Ouvre la liste des langues. Choisissez parmi 64 langues.", "Otwiera listę języków. Wybierz spośród 64 języków.", "Открывает список языков. Доступно 64 языка.", "Abre a lista de idiomas. Escolha entre 64 idiomas.", "Apre l'elenco delle lingue. Scegli tra 64 lingue.", "Opent de lijst met talen. Kies uit 64 talen.", "Deschide lista de limbi. Alegeți dintre 64 de limbi.", "Megnyitja a nyelvek listáját. Válasszon 64 nyelv közül.", "Öppnar språklistan. Välj bland 64 språk.", "Otevře seznam jazyků. Vyberte z 64 jazyků.", "Obre la llista d'idiomes. Tria entre 64 idiomes.", "打开语言列表。可从 64 种语言中选择。", "भाषाओं की सूची खोलता है। 64 भाषाओं में से चुनें।", "يفتح قائمة اللغات. اختر من بين 64 لغة.", "ভাষার তালিকা খোলে। ৬৪টি ভাষা থেকে বেছে নিন।", "زبانوں کی فہرست کھولتا ہے۔ 64 زبانوں میں سے چنیں۔", "Membuka daftar bahasa. Pilih dari 64 bahasa.", "Hufungua orodha ya lugha. Chagua kati ya lugha 64.", "言語の一覧を開きます。64言語から選べます。", "ਭਾਸ਼ਾਵਾਂ ਦੀ ਸੂਚੀ ਖੋਲ੍ਹਦਾ ਹੈ। 64 ਭਾਸ਼ਾਵਾਂ ਵਿੱਚੋਂ ਚੁਣੋ।", "भाषांची यादी उघडते. 64 भाषांमधून निवडा.", "భాషల జాబితాను తెరుస్తుంది. 64 భాషల నుండి ఎంచుకోండి.", "Mở danh sách ngôn ngữ. Chọn trong 64 ngôn ngữ.", "Mbukak dhaftar basa. Pilih saka 64 basa.", "", "Binubuksan ang listahan ng mga wika. Pumili mula sa 64 wika.", "فهرست زبان‌ها را باز می‌کند. از میان ۶۴ زبان انتخاب کنید.", "언어 목록을 엽니다. 64개 언어 중에서 선택하세요.", "மொழிகளின் பட்டியலைத் திறக்கிறது. 64 மொழிகளில் இருந்து தேர்ந்தெடுக்கவும்.", "Yana buɗe jerin harsuna. Zaɓi daga harsuna 64.", "ભાષાઓની યાદી ખોલે છે. 64 ભાષાઓમાંથી પસંદ કરો.", "เปิดรายการภาษา เลือกได้จาก 64 ภาษา", "ಭಾಷೆಗಳ ಪಟ್ಟಿಯನ್ನು ತೆರೆಯುತ್ತದೆ. 64 ಭಾಷೆಗಳಿಂದ ಆರಿಸಿ.", "", "", "", "", "ဘာသာစကားစာရင်းကို ဖွင့်သည်။ ဘာသာစကား ၆၄ ခုမှ ရွေးပါ။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_quit":       {"Closes Lumistar and removes its tray icon.", "Beendet Lumistar und entfernt das Symbol aus dem Infobereich.", "Lumistar'ı kapatır ve tepsi simgesini kaldırır.", "Lumistar-ı bağlayır və tepsi işarəsini silir.", "Cierra Lumistar y quita su icono de la bandeja.", "Ferme Lumistar et retire son icône de la zone de notification.", "Zamyka Lumistar i usuwa jego ikonę z obszaru powiadomień.", "Закрывает Lumistar и удаляет его значок из трея.", "Fecha o Lumistar e remove o ícone da bandeja.", "Chiude Lumistar e rimuove l'icona dall'area di notifica.", "Sluit Lumistar en verwijdert het pictogram uit het systeemvak.", "Închide Lumistar și elimină pictograma din zona de notificare.", "Bezárja a Lumistart, és eltávolítja a tálcaikont.", "Stänger Lumistar och tar bort ikonen från aktivitetsfältet.", "Ukončí Lumistar a odstraní ikonu z oznamovací oblasti.", "Tanca Lumistar i elimina la seva icona de la safata.", "关闭 Lumistar 并移除托盘图标。", "Lumistar बंद करता है और ट्रे आइकन हटा देता है।", "يغلق Lumistar ويزيل أيقونته من علبة النظام.", "Lumistar বন্ধ করে এবং ট্রে আইকন সরিয়ে দেয়।", "Lumistar بند کرتا ہے اور ٹرے آئیکن ہٹا دیتا ہے۔", "Menutup Lumistar dan menghapus ikonnya dari baki.", "Hufunga Lumistar na kuondoa aikoni yake kwenye trei.", "Lumistarを終了し、トレイアイコンを削除します。", "Lumistar ਬੰਦ ਕਰਦਾ ਹੈ ਅਤੇ ਟ੍ਰੇ ਆਈਕਨ ਹਟਾ ਦਿੰਦਾ ਹੈ।", "Lumistar बंद करते आणि ट्रे आयकॉन काढून टाकते.", "Lumistar ను మూసివేసి ట్రే ఐకాన్‌ను తొలగిస్తుంది.", "Đóng Lumistar và gỡ biểu tượng khỏi khay.", "Nutup Lumistar lan mbusak ikoné saka baki.", "", "Isinasara ang Lumistar at inaalis ang icon nito sa tray.", "Lumistar را می‌بندد و نماد سینی را حذف می‌کند.", "Lumistar를 종료하고 트레이 아이콘을 제거합니다.", "Lumistar ஐ மூடி ட்ரே ஐகானை அகற்றுகிறது.", "Yana rufe Lumistar kuma yana cire alamarsa daga tire.", "Lumistar બંધ કરે છે અને ટ્રે આઇકન દૂર કરે છે.", "ปิด Lumistar และลบไอคอนในเทรย์", "Lumistar ಮುಚ್ಚಿ ಟ್ರೇ ಐಕಾನ್ ತೆಗೆದುಹಾಕುತ್ತದೆ.", "", "", "", "", "Lumistar ကို ပိတ်ပြီး tray အိုင်ကွန်ကို ဖယ်ရှားသည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"nextpage":     {"Next Page", "Nächste Seite", "Sonraki Sayfa", "Növbəti səhifə", "Página siguiente", "Page suivante", "Następna strona", "Следующая страница", "Próxima página", "Pagina successiva", "Volgende pagina", "Pagina următoare", "Következő oldal", "Nästa sida", "Další stránka", "Pàgina següent", "下一页", "अगला पृष्ठ", "الصفحة التالية", "পরবর্তী পৃষ্ঠা", "اگلا صفحہ", "Halaman berikutnya", "Ukurasa unaofuata", "次のページ", "ਅਗਲਾ ਪੰਨਾ", "पुढील पृष्ठ", "తదుపరి పేజీ", "Trang sau", "Kaca sabanjure", "Halaman seterusnya", "Susunod na pahina", "صفحه بعد", "다음 페이지", "அடுத்த பக்கம்", "Shafi na gaba", "આગલું પૃષ્ઠ", "หน้าถัดไป", "ಮುಂದಿನ ಪುಟ", "بله پاڼه", "ቀጣይ ገጽ", "", "Ojú-ìwé tó kàn", "နောက်စာမျက်နှာ", "", "Наступна сторінка", "", "അടുത്ത പേജ്", "", "Keyingi sahifa", "", "अर्को पृष्ठ", "Ibe na-esote", "", "Rûpela paşîn", "", "Ikhasi elilandelayo", "", "Bogga xiga", "Iphepha elilandelayo", "Volgende bladsy", "ඊළඟ පිටුව", "ទំព័របន្ទាប់", "Келесі бет", "Επόμενη σελίδα", ""},
	"prevpage":     {"Previous Page", "Vorherige Seite", "Önceki Sayfa", "Əvvəlki səhifə", "Página anterior", "Page précédente", "Poprzednia strona", "Предыдущая страница", "Página anterior", "Pagina precedente", "Vorige pagina", "Pagina anterioară", "Előző oldal", "Föregående sida", "Předchozí stránka", "Pàgina anterior", "上一页", "पिछला पृष्ठ", "الصفحة السابقة", "আগের পৃষ্ঠা", "پچھلا صفحہ", "Halaman sebelumnya", "Ukurasa uliotangulia", "前のページ", "ਪਿਛਲਾ ਪੰਨਾ", "मागील पृष्ठ", "మునుపటి పేజీ", "Trang trước", "Kaca sadurunge", "Halaman sebelumnya", "Nakaraang pahina", "صفحه قبل", "이전 페이지", "முந்தைய பக்கம்", "Shafi na baya", "પાછલું પૃષ્ઠ", "หน้าก่อนหน้า", "ಹಿಂದಿನ ಪುಟ", "تېره پاڼه", "ቀዳሚ ገጽ", "", "Ojú-ìwé tó ṣáájú", "ယခင်စာမျက်နှာ", "", "Попередня сторінка", "", "മുൻ പേജ്", "", "Oldingi sahifa", "", "अघिल्लो पृष्ठ", "Ibe gara aga", "", "Rûpela berê", "", "Ikhasi elidlule", "", "Bogga hore", "Iphepha langaphambili", "Vorige bladsy", "පෙර පිටුව", "ទំព័រមុន", "Алдыңғы бет", "Προηγούμενη σελίδα", ""},
	"help":         {"Help", "Hilfe", "Yardım", "Kömək", "Ayuda", "Aide", "Pomoc", "Справка", "Ajuda", "Aiuto", "Help", "Ajutor", "Súgó", "Hjälp", "Nápověda", "Ajuda", "帮助", "सहायता", "مساعدة", "সাহায্য", "مدد", "Bantuan", "Msaada", "ヘルプ", "ਮਦਦ", "मदत", "సహాయం", "Trợ giúp", "Pitulung", "Bantuan", "Tulong", "راهنما", "도움말", "உதவி", "Taimako", "મદદ", "ช่วยเหลือ", "ಸಹಾಯ", "مرسته", "እገዛ", "", "Ìrànlọ́wọ́", "အကူအညီ", "", "Довідка", "", "സഹായം", "", "Yordam", "", "मद्दत", "Enyemaka", "", "Alîkarî", "", "Usizo", "", "Caawimo", "Uncedo", "Hulp", "උදවු", "ជំនួយ", "Анықтама", "Βοήθεια", ""},
	"d_sound":      {"Opens the Windows Sound settings.", "Öffnet die Windows-Soundeinstellungen.", "Windows Ses ayarlarını açar.", "Windows Səs ayarlarını açır.", "Abre la configuración de sonido de Windows.", "Ouvre les paramètres audio de Windows.", "Otwiera ustawienia dźwięku systemu Windows.", "Открывает параметры звука Windows.", "Abre as configurações de Som do Windows.", "Apre le impostazioni audio di Windows.", "Opent de Windows-geluidsinstellingen.", "Deschide setările Windows pentru Sunet.", "Megnyitja a Windows Hang beállításait.", "Öppnar Windows ljudinställningar.", "Otevře nastavení Zvuku ve Windows.", "Obre la configuració de So de Windows.", "打开 Windows 声音设置。", "Windows साउंड सेटिंग खोलता है।", "يفتح إعدادات الصوت في Windows.", "Windows সাউন্ড সেটিংস খোলে।", "Windows ساؤنڈ سیٹنگز کھولتا ہے۔", "Membuka pengaturan Suara Windows.", "Hufungua mipangilio ya Sauti ya Windows.", "Windowsのサウンド設定を開きます。", "Windows ਸਾਊਂਡ ਸੈਟਿੰਗਾਂ ਖੋਲ੍ਹਦਾ ਹੈ।", "Windows साउंड सेटिंग्ज उघडते.", "Windows సౌండ్ సెట్టింగ్‌లను తెరుస్తుంది.", "Mở cài đặt Âm thanh của Windows.", "Mbukak setelan Swara Windows.", "", "Binubuksan ang mga setting ng Sound ng Windows.", "تنظیمات صدای ویندوز را باز می‌کند.", "Windows 소리 설정을 엽니다.", "Windows ஒலி அமைப்புகளைத் திறக்கிறது.", "Yana buɗe saitunan Sauti na Windows.", "Windows સાઉન્ડ સેટિંગ્સ ખોલે છે.", "เปิดการตั้งค่าเสียงของ Windows", "Windows ಸೌಂಡ್ ಸೆಟ್ಟಿಂಗ್‌ಗಳನ್ನು ತೆರೆಯುತ್ತದೆ.", "", "", "", "", "Windows အသံဆက်တင်များကို ဖွင့်သည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	"d_snip":       {"Starts the Windows screenshot tool (Win + Shift + S).", "Startet das Windows-Screenshot-Tool (Win + Umschalt + S).", "Windows ekran görüntüsü aracını (Win + Shift + S) başlatır.", "Windows ekran görüntüsü alətini (Win + Shift + S) işə salır.", "Inicia la herramienta de capturas de Windows (Win + Mayús + S).", "Lance l'outil de capture d'écran de Windows (Win + Maj + S).", "Uruchamia narzędzie do zrzutów ekranu systemu Windows (Win + Shift + S).", "Запускает средство создания снимков экрана Windows (Win + Shift + S).", "Inicia a ferramenta de captura de tela do Windows (Win + Shift + S).", "Avvia lo strumento di cattura di Windows (Win + Maiusc + S).", "Start de Windows-schermopnametool (Win + Shift + S).", "Pornește instrumentul de capturi de ecran Windows (Win + Shift + S).", "Elindítja a Windows képernyőkép-eszközt (Win + Shift + S).", "Startar Windows skärmklippverktyg (Win + Skift + S).", "Spustí nástroj pro snímání obrazovky ve Windows (Win + Shift + S).", "Inicia l'eina de captures de pantalla de Windows (Win + Maj + S).", "启动 Windows 截图工具 (Win + Shift + S)。", "Windows स्क्रीनशॉट टूल (Win + Shift + S) शुरू करता है।", "يشغّل أداة لقطة الشاشة في Windows (Win + Shift + S).", "Windows স্ক্রিনশট টুল (Win + Shift + S) চালু করে।", "Windows اسکرین شاٹ ٹول (Win + Shift + S) شروع کرتا ہے۔", "Menjalankan alat tangkapan layar Windows (Win + Shift + S).", "Huanzisha zana ya picha ya skrini ya Windows (Win + Shift + S).", "Windowsのスクリーンショットツール（Win + Shift + S）を起動します。", "Windows ਸਕ੍ਰੀਨਸ਼ਾਟ ਟੂਲ (Win + Shift + S) ਸ਼ੁਰੂ ਕਰਦਾ ਹੈ।", "Windows स्क्रीनशॉट टूल (Win + Shift + S) सुरू करते.", "Windows స్క్రీన్‌షాట్ టూల్ (Win + Shift + S) ను ప్రారంభిస్తుంది.", "Khởi động công cụ chụp màn hình của Windows (Win + Shift + S).", "Miwiti piranti tangkapan layar Windows (Win + Shift + S).", "", "Sinisimulan ang screenshot tool ng Windows (Win + Shift + S).", "ابزار عکس‌برداری از صفحه ویندوز (Win + Shift + S) را اجرا می‌کند.", "Windows 화면 캡처 도구(Win + Shift + S)를 시작합니다.", "Windows ஸ்கிரீன்ஷாட் கருவியை (Win + Shift + S) தொடங்குகிறது.", "Yana fara kayan aikin ɗaukar hoton allo na Windows (Win + Shift + S).", "Windows સ્ક્રીનશોટ ટૂલ (Win + Shift + S) શરૂ કરે છે.", "เริ่มเครื่องมือจับภาพหน้าจอของ Windows (Win + Shift + S)", "Windows ಸ್ಕ್ರೀನ್‌ಶಾಟ್ ಟೂಲ್ (Win + Shift + S) ಆರಂಭಿಸುತ್ತದೆ.", "", "", "", "", "Windows ဖန်သားပြင်ဓာတ်ပုံရိုက်ကိရိယာ (Win + Shift + S) ကို စတင်သည်။", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
}

func selectLang(id string) {
	i, err := strconv.Atoi(id[1:])
	if err != nil || i < 0 || i >= len(langs) {
		return
	}
	mu.Lock()
	lang = i // the language page stays open until Esc, a click elsewhere or Back
	mu.Unlock()
	saveTheme(false)
	relayout()
}

// ---------------------------------------------------------------- microphone and lock

var pLockWS = user32.NewProc("LockWorkStation")

func getMic() (muted, ok bool) {
	audioDoFlow(1, func(ep uintptr) {
		var m int32
		if int32(vcall(ep, 15, ptr(&m))) >= 0 {
			muted, ok = m != 0, true
		}
	})
	return
}

func toggleMic() {
	mu.Lock()
	micMuted = !micMuted
	m := micMuted
	mu.Unlock()
	go audioDoFlow(1, func(ep uintptr) {
		b := uintptr(0)
		if m {
			b = 1
		}
		vcall(ep, 14, b, 0)
	})
}

func lockPC() {
	hidePanel()
	call(pLockWS)
}

// ---------------------------------------------------------------- icons (microphone, GitHub, Claude) and links

//go:embed gh_white.raw
var ghWhiteRaw []byte

//go:embed gh_dark.raw
var ghDarkRaw []byte

//go:embed claude.raw
var claudeRaw []byte

const (
	repoURL   = "https://github.com/fuwifur/lumistar-tray"
	claudeURL = "https://claude.ai"
)

var (
	ghWhiteImg, ghDarkImg, claudeImg uintptr
	pGdipPen                         = gdiplus.NewProc("GdipCreatePen1")
	pGdipPenCap                      = gdiplus.NewProc("GdipSetPenLineCap197819")
	pGdipDelPen                      = gdiplus.NewProc("GdipDeletePen")
	pGdipArc                         = gdiplus.NewProc("GdipDrawArcI")
	pGdipLine                        = gdiplus.NewProc("GdipDrawLineI")
)

func f32(v float64) uintptr { return uintptr(math.Float32bits(float32(v))) }

func (p *painter) pen(col uint32, width float64) uintptr {
	var pen uintptr
	call(pGdipPen, uintptr(0xFF000000|col), f32(width), 2, ptr(&pen))
	call(pGdipPenCap, pen, 2, 2, 0)
	return pen
}

func (p *painter) arc(pen uintptr, x, y, w, h int, start, sweep float64) {
	call(pGdipArc, p.g, pen, iu(x), iu(y), iu(w), iu(h), f32(start), f32(sweep))
}

func (p *painter) line(pen uintptr, x1, y1, x2, y2 int) {
	call(pGdipLine, p.g, pen, iu(x1), iu(y1), iu(x2), iu(y2))
}

// mic draws a hollow outline microphone; when muted a line crosses it.
func (p *painter) mic(r rect, col uint32, muted bool) {
	pen := p.pen(col, 1.5*float64(scale)/100)
	cx, cy := int(r.l+r.r)/2, int(r.t+r.b)/2
	w, h := px(7), px(11)
	top := cy - px(8)
	x0 := cx - w/2
	p.arc(pen, x0, top, w, w, 180, 180)
	p.arc(pen, x0, top+h-w, w, w, 0, 180)
	p.line(pen, x0, top+w/2, x0, top+h-w/2)
	p.line(pen, x0+w, top+w/2, x0+w, top+h-w/2)
	uw := px(11)
	p.arc(pen, cx-uw/2, top+px(3), uw, px(10), 0, 180)
	p.line(pen, cx, top+px(13), cx, top+px(16))
	p.line(pen, cx-px(3), top+px(16), cx+px(3), top+px(16))
	if muted {
		p.line(pen, cx+px(7), cy-px(8), cx-px(7), cy+px(8)) // top right to bottom left
	}
	call(pGdipDelPen, pen)
}

// icon draws the picture on an icon button: 1 microphone, 2 GitHub, 3 Claude, and the hollow outlined symbols
// 6 speaker, 7 gear, 11 screenshot.
// bg is the colour of the button behind the picture.
func (p *painter) icon(it item, light bool, pal palette, bg uint32) {
	cx, cy := int(it.r.l+it.r.r)/2, int(it.r.t+it.r.b)/2
	col := uint32(0xffffff)
	if light && !it.on {
		col = pal.text
	}
	switch it.icon {
	case 1:
		p.mic(it.r, col, it.on)
	case 2, 3:
		img := claudeImg
		if it.icon == 2 {
			img = ghWhiteImg
			if light {
				img = ghDarkImg
			}
		}
		sz := px(18)
		call(pGdipInterp, p.g, 7)
		call(pGdipDrawImg, p.g, img, iu(cx-sz/2), iu(cy-sz/2), uintptr(sz), uintptr(sz))
	default:
		if it.icon >= 4 {
			p.outline(it.icon, cx, cy, col, bg, it.on)
		}
	}
}

// flag draws the small country flag at the left of a language button.
func (p *painter) flag(it item) {
	img := flagImgs[it.flag]
	if img == 0 {
		return
	}
	fw, fh := px(20), px(15)
	call(pGdipInterp, p.g, 7)
	call(pGdipDrawImg, p.g, img, iu(int(it.r.l)+px(10)), iu(int(it.r.t+it.r.b)/2-fh/2), uintptr(fw), uintptr(fh))
}

// ---------------------------------------------------------------- flags, new icons, safety net

//go:embed flags.raw
var flagsRaw []byte

var (
	flagCodes    = []string{"gb", "us", "de", "tr", "az", "es", "fr", "pl", "ru", "pt", "it", "sa", "id", "bd", "cn", "in", "pk", "jp", "vn", "ng", "tz", "ph", "ir", "kr", "th", "mm", "uz", "kz", "tm", "kg", "ua", "md", "nl", "ro", "hu", "se", "cz", "es-ct", "af", "et", "gr", "iq", "kh", "lk", "mg", "my", "np", "so", "za"}
	flagImgs     = map[string]uintptr{}
	pGdipRectOut = gdiplus.NewProc("GdipDrawRectangleI")
	escapeSink   unsafe.Pointer
	keepStr      = map[string]*uint16{}
)

// loadFlags turns the embedded 40x30 flag pictures into GDI+ bitmaps (once, at start-up).
func loadFlags() {
	const size = 40 * 30 * 4
	for i, c := range flagCodes {
		if (i+1)*size > len(flagsRaw) {
			break
		}
		var img uintptr
		call(pGdipBitmap, 40, 30, 40*4, 0x26200A, uintptr(unsafe.Pointer(&flagsRaw[i*size])), ptr(&img))
		flagImgs[c] = img
	}
}

// strPtr returns a Windows string pointer that stays alive for the whole run (start-up only).
func strPtr(s string) uintptr {
	p, ok := keepStr[s]
	if !ok {
		p = u16(s)
		keepStr[s] = p
	}
	return uintptr(unsafe.Pointer(p))
}

// guard is deferred in every callback and goroutine: instead of letting an unexpected error close Lumistar,
// it writes the details to %LOCALAPPDATA%\Lumistar\crash.log and keeps the program running.
func guard() {
	if r := recover(); r != nil {
		logPanic(r)
	}
}

func logPanic(r any) {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Lumistar")
	os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "crash.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  panic: %v\n%s\n", time.Now().Format(time.RFC3339), r, debug.Stack())
}

// ---------------------------------------------------------------- hollow outlined icons, hotkeys, help scrolling

var (
	workH           int // tallest panel that fits the screen
	helpScroll      int // how far the help page is scrolled
	pGdipPolyOut    = gdiplus.NewProc("GdipDrawPolygonI")
	pGdipEllipseOut = gdiplus.NewProc("GdipDrawEllipseI")
)

// outline draws a hollow symbol centred on (cx, cy). slash adds the muted line (top right to bottom left).
func (p *painter) outline(icon, cx, cy int, col, bg uint32, slash bool) {
	s := float64(scale) / 100
	pen := p.pen(col, 1.5*s)
	switch icon {
	case 6: // speaker with sound waves, centred
		ox := cx - px(1)
		pts := make([]point, 6)
		for i, d := range [][2]int{{-7, -3}, {-4, -3}, {0, -7}, {0, 7}, {-4, 3}, {-7, 3}} {
			pts[i] = point{int32(ox + px(d[0])), int32(cy + px(d[1]))}
		}
		call(pGdipPolyOut, p.g, pen, ptr(&pts[0]), 6)
		p.arc(pen, ox-px(5), cy-px(5), px(10), px(10), -50, 100)
		p.arc(pen, ox-px(9), cy-px(9), px(18), px(18), -50, 100)
	case 7: // gear: toothed outline with a hollow centre, like the Windows settings gear
		const teeth = 8
		ro, ri := 8*s, 6.3*s
		pts := make([]point, 0, teeth*4)
		for k := 0; k < teeth; k++ {
			base := float64(k) * 2 * math.Pi / teeth
			for _, c := range []struct{ a, r float64 }{{-0.32, ri}, {-0.17, ro}, {0.17, ro}, {0.32, ri}} {
				ang := base + c.a
				pts = append(pts, point{int32(cx + int(math.Round(math.Cos(ang)*c.r))), int32(cy + int(math.Round(math.Sin(ang)*c.r)))})
			}
		}
		call(pGdipPolyOut, p.g, pen, ptr(&pts[0]), uintptr(len(pts)))
		hr := int(math.Round(2.6 * s))
		call(pGdipEllipseOut, p.g, pen, iu(cx-hr), iu(cy-hr), iu(2*hr), iu(2*hr))
	case 11: // screenshot: four corner brackets
		x0, x1, y0, y1, l := cx-px(7), cx+px(7), cy-px(6), cy+px(6), px(4)
		p.line(pen, x0, y0, x0+l, y0)
		p.line(pen, x0, y0, x0, y0+l)
		p.line(pen, x1, y0, x1-l, y0)
		p.line(pen, x1, y0, x1, y0+l)
		p.line(pen, x0, y1, x0+l, y1)
		p.line(pen, x0, y1, x0, y1-l)
		p.line(pen, x1, y1, x1-l, y1)
		p.line(pen, x1, y1, x1, y1-l)
	}
	if slash && icon == 6 {
		p.line(pen, cx+px(8), cy-px(8), cx-px(8), cy+px(8))
	}
	call(pGdipDelPen, pen)
}
