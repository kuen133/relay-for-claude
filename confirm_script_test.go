//go:build darwin

package main

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The dialog's AppleScript must compile; osacompile checks it without showing anything.
func TestKuenConfirmScriptCompiles(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en"} {
		script, _ := dialogScript(lang, restartWarning(lang, "Add account", 2*time.Minute))
		out := filepath.Join(t.TempDir(), "check.scpt")
		if b, err := exec.Command("osacompile", "-o", out, "-e", script).CombinedOutput(); err != nil {
			t.Fatalf("%s script does not compile: %v %s", lang, err, b)
		}
	}
}
