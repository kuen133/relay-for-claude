package main

import (
	"fmt"
	"strings"
	"time"
)

// Kuen: adding an account or importing chats restarts Claude, which stops any
// task running in it. When a chat was written recently, ask first.

// recentWork says how long ago Claude last wrote a chat, and whether that is
// recent enough that a task may still be running.
func (s *switcher) recentWork(now time.Time) (time.Duration, bool) {
	if !desktopRunning() {
		return 0, false
	}
	last := lastChatWrite(s.app().sessionsDir())
	if last.IsZero() {
		return 0, false
	}
	ago := now.Sub(last)
	return ago, ago < idleFor
}

// restartWarning is the question shown before a restart that may cut off a task.
func restartWarning(lang, what string, ago time.Duration) string {
	mins := int(ago.Minutes())
	if lang == "zh-CN" {
		when := "刚刚"
		if mins > 0 {
			when = fmt.Sprintf("%d 分钟前", mins)
		}
		return fmt.Sprintf("Claude %s还在写会话记录，可能有任务正在运行。\n\n%s需要重启 Claude，正在运行的任务会被中断。确定继续吗？", when, what)
	}
	when := "just now"
	if mins > 0 {
		when = fmt.Sprintf("%d min ago", mins)
	}
	return fmt.Sprintf("Claude was still writing a chat %s, a task may be running.\n\n%s restarts Claude and stops that task. Continue?", when, what)
}

// guardedRun runs a restarting action, asking first while Claude looks busy.
func (t *tray) guardedRun(what string, f func() error) {
	go func() {
		snap := t.s.snapshot()
		if ago, busy := t.s.recentWork(time.Now()); busy {
			name := translate(snap.Language, what)
			if !confirmDialog(snap.Language, restartWarning(snap.Language, name, ago)) {
				return
			}
		}
		t.s.run(what, f)
	}()
}

// appleString quotes s as an AppleScript string literal.
func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
