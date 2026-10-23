//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
#include "panel_darwin.h"
*/
import "C"

import (
	_ "embed"
	"encoding/json"
	"log"
	"unsafe"
)

//go:embed panel.html
var panelHTML string

const hasGlassPanel = true

// panelHandler receives the page's commands; the tray sets it.
var panelHandler func(panelCommand)

func glassInit(handle func(panelCommand)) {
	panelHandler = handle
	html := C.CString(panelHTML)
	defer C.free(unsafe.Pointer(html))
	C.panelInit(html)
}

func glassToggle() { C.panelToggle() }
func glassHide()   { C.panelHide() }

func glassUpdate(st panelState) {
	data, err := json.Marshal(st)
	if err != nil {
		log.Printf("panel state: %v", err)
		return
	}
	s := C.CString(string(data))
	defer C.free(unsafe.Pointer(s))
	C.panelUpdate(s)
}

//export panelMessage
func panelMessage(raw *C.char) {
	c, err := parsePanelCommand(C.GoString(raw))
	if err != nil || panelHandler == nil {
		return
	}
	// Commands can restart Claude, which takes seconds; keep the UI thread free.
	go panelHandler(c)
}
