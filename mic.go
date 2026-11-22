package main

import "time"

// Microphone mute lock. The microphone button mutes the default microphone. While Lumistar holds it muted,
// micGuard checks the device every 1.5 seconds and mutes it again if Windows or another program unmuted it.
// Only the microphone button unlocks it. The microphone level is never changed.

var micLocked bool

// toggleMic is the microphone button: it locks the microphone when it is open and unlocks it when it is muted.
func toggleMic() {
	mu.Lock()
	muted := micMuted
	micLocked = !muted
	micMuted = !muted
	mu.Unlock()
	go micSetMute(!muted)
}

// micSetMute mutes or unmutes the default microphone.
func micSetMute(mute bool) {
	audioDoFlow(1, func(ep uintptr) {
		b := uintptr(0)
		if mute {
			b = 1
		}
		vcall(ep, 14, b, 0)
	})
}

// micGuard mutes a locked microphone again whenever something else has unmuted it.
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
			vcall(ep, 15, ptr(&m))
			if m == 0 {
				vcall(ep, 14, 1, 0)
			}
		})
	}
}
