//go:build !darwin

package main

// confirmDialog has no prompt on other platforms yet, so actions go ahead as before.
func confirmDialog(lang, message string) bool { return true }
