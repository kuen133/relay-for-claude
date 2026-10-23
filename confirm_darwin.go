//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

// confirmDialog shows a native warning with Cancel as the default button and
// reports whether the user chose to continue. Closing it counts as Cancel.
func confirmDialog(lang, message string) bool {
	script, proceed := dialogScript(lang, message)
	out, err := exec.Command("osascript", "-e", script).Output()
	return err == nil && strings.Contains(string(out), proceed)
}

// dialogScript builds the AppleScript for the dialog and names its continue button.
func dialogScript(lang, message string) (script, proceed string) {
	cancel, proceed, title := "Cancel", "Continue anyway", "Relay for Claude"
	if lang == "zh-CN" {
		cancel, proceed, title = "取消", "仍然继续", "额度接力 Relay"
	}
	script = `display dialog ` + appleString(message) +
		` with title ` + appleString(title) +
		` buttons {` + appleString(cancel) + `, ` + appleString(proceed) + `}` +
		` default button ` + appleString(cancel) + ` cancel button ` + appleString(cancel) + ` with icon caution`
	return script, proceed
}
