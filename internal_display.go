//go:build windows

package main

// Built-in laptop panels through WMI (namespace ROOT\WMI), called directly over COM.
// No child process is started: no PowerShell, no cmd, no hidden windows.
// All calls run on the audio thread, which is already initialised for COM (multithreaded).

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	oleaut32         = syscall.NewLazyDLL("oleaut32.dll")
	pSysAllocString  = oleaut32.NewProc("SysAllocString")
	pSysFreeString   = oleaut32.NewProc("SysFreeString")
	pVariantClear    = oleaut32.NewProc("VariantClear")
	pProxyBlanket    = ole32.NewProc("CoSetProxyBlanket")
	clsidWbemLocator = guid{0x4590F811, 0x1D3A, 0x11D0, [8]byte{0x89, 0x1F, 0x00, 0xAA, 0x00, 0x4B, 0x2E, 0x24}}
	iidWbemLocator   = guid{0xDC12A687, 0x737F, 0x11CF, [8]byte{0x88, 0x4D, 0x00, 0xAA, 0x00, 0x4B, 0x2E, 0x24}}
)

const (
	vtI4   = 3  // VT_I4
	vtUI1  = 17 // VT_UI1
	vtBSTR = 8  // VT_BSTR
	cimUI1 = 17 // CIM_UINT8
	cimUI4 = 19 // CIM_UINT32
)

// variant is a 24-byte VARIANT: the type at offset 0, the value at offset 8 on 64-bit Windows.
type variant [24]byte

func (v *variant) kind() uint16 { return binary.LittleEndian.Uint16(v[0:]) }

func varI4(x uint32) variant {
	var v variant
	binary.LittleEndian.PutUint16(v[0:], vtI4)
	binary.LittleEndian.PutUint32(v[8:], x)
	return v
}

func varUI1(x byte) variant {
	var v variant
	binary.LittleEndian.PutUint16(v[0:], vtUI1)
	v[8] = x
	return v
}

// wbem holds the connection. Only touched on the audio thread.
var wbem struct {
	svc     uintptr // IWbemServices
	methods uintptr // instance of WmiMonitorBrightnessMethods
	path    uintptr // BSTR: object path of that instance
	failed  bool    // connecting failed (for example on a desktop PC); do not retry
}

// wbemRun runs f on the audio thread and waits for it.
func wbemRun(f func()) {
	done := make(chan struct{})
	audioCh <- func() {
		defer close(done)
		f()
	}
	<-done
}

// internalGet returns the brightness of the built-in panel, if the PC has one.
func internalGet() (int, bool) {
	var v int
	var ok bool
	wbemRun(func() {
		if wbemConnect() {
			v, ok = wbemReadBrightness()
		}
	})
	return v, ok
}

// internalSet sets the built-in panel brightness (0-100).
func internalSet(level int) bool {
	var ok bool
	wbemRun(func() {
		if wbemConnect() && wbemPrepareMethods() {
			ok = wbemWriteBrightness(level)
		}
	})
	return ok
}

func bstrAlloc(s string) uintptr {
	p := u16(s)
	r, _, _ := pSysAllocString.Call(uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	return r
}

func bstrFree(b uintptr) {
	if b != 0 {
		pSysFreeString.Call(b)
	}
}

func wbemConnect() bool {
	if wbem.svc != 0 {
		return true
	}
	if wbem.failed {
		return false
	}
	var loc uintptr
	// CoCreateInstance(CLSID_WbemLocator, nil, CLSCTX_INPROC_SERVER, IID_IWbemLocator, &loc)
	if int32(call(pCoCreate, ptr(&clsidWbemLocator), 0, 1, ptr(&iidWbemLocator), ptr(&loc))) < 0 || loc == 0 {
		wbem.failed = true
		return false
	}
	ns := bstrAlloc(`ROOT\WMI`)
	var svc uintptr
	// IWbemLocator::ConnectServer(network, user, password, locale, flags, authority, ctx, &svc)
	hr := vcall(loc, 3, ns, 0, 0, 0, 0, 0, 0, ptr(&svc))
	bstrFree(ns)
	vcall(loc, 2) // Release
	if int32(hr) < 0 || svc == 0 {
		wbem.failed = true
		return false
	}
	// Call-level authentication on the proxy, as WMI clients normally set it.
	call(pProxyBlanket, svc, 10, 0, 0, 3, 3, 0, 0)
	wbem.svc = svc
	return true
}

// wbemFirst runs a WQL query and returns its first object (IWbemClassObject), or 0.
func wbemFirst(query string) uintptr {
	lang, q := bstrAlloc("WQL"), bstrAlloc(query)
	var en uintptr
	// IWbemServices::ExecQuery(language, query, flags, ctx, &enumerator)
	hr := vcall(wbem.svc, 20, lang, q, 0, 0, ptr(&en))
	bstrFree(lang)
	bstrFree(q)
	if int32(hr) < 0 || en == 0 {
		return 0
	}
	var obj uintptr
	var got uint32
	// IEnumWbemClassObject::Next(timeout, count, &obj, &returned)
	hr = vcall(en, 4, 0xFFFFFFFF, 1, ptr(&obj), ptr(&got))
	vcall(en, 2) // Release
	if int32(hr) < 0 || got == 0 || obj == 0 {
		return 0
	}
	return obj
}

// wbemGet reads one property of an object.
func wbemGet(obj uintptr, name string) (variant, bool) {
	var v variant
	n := u16(name)
	// IWbemClassObject::Get(name, flags, &value, &type, &flavor)
	hr := vcall(obj, 4, uintptr(unsafe.Pointer(n)), 0, ptr(&v), 0, 0)
	runtime.KeepAlive(n)
	return v, int32(hr) >= 0
}

func wbemReadBrightness() (int, bool) {
	obj := wbemFirst("SELECT * FROM WmiMonitorBrightness")
	if obj == 0 {
		return 0, false
	}
	defer vcall(obj, 2)
	v, ok := wbemGet(obj, "CurrentBrightness")
	if !ok || v.kind() != vtUI1 {
		return 0, false
	}
	return int(v[8]), true
}

// wbemPrepareMethods finds the WmiMonitorBrightnessMethods instance once and keeps it.
func wbemPrepareMethods() bool {
	if wbem.methods != 0 {
		return true
	}
	obj := wbemFirst("SELECT * FROM WmiMonitorBrightnessMethods")
	if obj == 0 {
		return false
	}
	v, ok := wbemGet(obj, "__PATH")
	if !ok || v.kind() != vtBSTR {
		vcall(obj, 2)
		return false
	}
	// Copy the path so it stays valid after VariantClear frees the original.
	src := uintptr(binary.LittleEndian.Uint64(v[8:]))
	r, _, _ := pSysAllocString.Call(src)
	pVariantClear.Call(ptr(&v))
	if r == 0 {
		vcall(obj, 2)
		return false
	}
	wbem.path = r
	wbem.methods = obj
	return true
}

func wbemWriteBrightness(level int) bool {
	method := bstrAlloc("WmiSetBrightness")
	defer bstrFree(method)
	mname := u16("WmiSetBrightness")
	var sig uintptr
	// IWbemClassObject::GetMethod(name, flags, &inSignature, &outSignature)
	hr := vcall(wbem.methods, 19, uintptr(unsafe.Pointer(mname)), 0, ptr(&sig), 0)
	runtime.KeepAlive(mname)
	if int32(hr) < 0 || sig == 0 {
		return false
	}
	defer vcall(sig, 2)
	var in uintptr
	// IWbemClassObject::SpawnInstance(flags, &instance)
	hr = vcall(sig, 15, 0, ptr(&in))
	if int32(hr) < 0 || in == 0 {
		return false
	}
	defer vcall(in, 2)
	timeout, brightness := varI4(1), varUI1(byte(level))
	tn, bn := u16("Timeout"), u16("Brightness")
	// IWbemClassObject::Put(name, flags, &value, type)
	vcall(in, 5, uintptr(unsafe.Pointer(tn)), 0, ptr(&timeout), cimUI4)
	vcall(in, 5, uintptr(unsafe.Pointer(bn)), 0, ptr(&brightness), cimUI1)
	runtime.KeepAlive(tn)
	runtime.KeepAlive(bn)
	// IWbemServices::ExecMethod(objectPath, methodName, flags, ctx, inParams, &outParams, &callResult)
	hr = vcall(wbem.svc, 24, wbem.path, method, 0, 0, in, 0, 0)
	return int32(hr) >= 0
}
