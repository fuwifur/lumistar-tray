package main

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Microphone lock. The microphone button mutes the default microphone and sets its level to 0.
// While Lumistar holds it muted, micGuard checks the device every 1.5 s and mutes it again if
// Windows or another program unmuted it or raised the level. Only the button unlocks it again,
// and then the saved level is restored. Quitting Lumistar keeps the microphone as it is.

var (
	micLocked bool
	micSaved  = -1 // level before the lock, -1 = unknown (from settings or never locked)
)

// micSavedLevel returns the saved level for settings.txt; 50 when nothing is known yet.
func micSavedLevel() int {
	mu.Lock()
	defer mu.Unlock()
	if micSaved < 0 {
		return 50
	}
	return micSaved
}

// loadMicLevel reads the saved level from the text of settings.txt.
func loadMicLevel(text string) {
	for _, line := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "miclevel="); ok {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100 {
				mu.Lock()
				micSaved = n
				mu.Unlock()
			}
		}
	}
}

// toggleMic is the microphone button: it locks the microphone when it is open and unlocks it when it is muted.
func toggleMic() {
	mu.Lock()
	muted := micMuted
	mu.Unlock()
	if muted {
		micUnlock()
	} else {
		micLock()
	}
}

func micLock() {
	level := 0
	audioDoFlow(1, func(ep uintptr) {
		var f float32
		if int32(vcall(ep, 9, ptr(&f))) >= 0 {
			level = int(math.Round(float64(f) * 100))
		}
	})
	mu.Lock()
	if !micLocked {
		micSaved = level
	}
	micLocked = true
	micMuted = true
	mu.Unlock()
	saveTheme(false)
	go micApply(true, 0)
}

func micUnlock() {
	mu.Lock()
	micLocked = false
	micMuted = false
	level := micSaved
	mu.Unlock()
	if level < 0 {
		level = 50
	}
	go micApply(false, level)
}

// micApply sets the mute state and the level of the default microphone.
func micApply(mute bool, level int) {
	audioDoFlow(1, func(ep uintptr) {
		b := uintptr(0)
		if mute {
			b = 1
		}
		vcall(ep, 14, b, 0)
		vcall(ep, 7, uintptr(math.Float32bits(float32(level)/100)), 0)
	})
}

// micGuard keeps a locked microphone muted at level 0.
func micGuard() {
	for {
		time.Sleep(1500 * time.Millisecond)
		mu.Lock()
		locked := micLocked
		mu.Unlock()
		if !locked {
			continue
		}
		audioDoFlow(1, func(ep uintptr) {
			var m int32
			var f float32
			vcall(ep, 15, ptr(&m))
			vcall(ep, 9, ptr(&f))
			if m == 0 {
				vcall(ep, 14, 1, 0)
			}
			if f > 0 {
				vcall(ep, 7, 0, 0)
			}
		})
	}
}
