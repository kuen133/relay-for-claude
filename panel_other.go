//go:build !darwin

package main

// Other platforms keep the plain tray menu.
const hasGlassPanel = false

func glassInit(func(panelCommand)) {}
func glassToggle()                 {}
func glassHide()                   {}
func glassUpdate(panelState)       {}
