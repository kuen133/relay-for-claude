package main

import (
	"strings"
	"testing"
	"time"
)

func TestKuenFiveHour93(t *testing.T) {
	now := time.Now()
	a := &Account{UUID: "a", Usage: &Usage{Session: 92.99, Weekly: 97, CheckedAt: now}}
	b := &Account{UUID: "b", Usage: &Usage{Session: 20, Weekly: 30, CheckedAt: now}}
	s := &switcher{active: "a", st: State{Accounts: []*Account{a, b}, AutoSwitch: true, Threshold: 93, FiveHourOnly: true}}
	if got, _ := s.autoTarget(true, now, now); got != nil {
		t.Fatal("weekly usage must not trigger a five-hour-only switch")
	}
	a.Usage.Session = 93
	if got, why := s.autoTarget(true, now, now); got != b || why != nearlyFull {
		t.Fatal("93 percent must select the available second account")
	}
	b.Usage.Weekly = 95
	if got, _ := s.autoTarget(true, now, now); got != nil {
		t.Fatal("must not select a nearly exhausted weekly allowance")
	}
	b.Usage.Weekly = 30
	a.Usage.CheckedAt = now.Add(-activeFresh - time.Second)
	if got, _ := s.autoTarget(true, now, now); got != nil {
		t.Fatal("stale readings must not trigger a switch")
	}
}

func TestKuenWeeklyBackstop(t *testing.T) {
	now := time.Now()
	a := &Account{UUID: "a", Usage: &Usage{Session: 10, Weekly: 98, CheckedAt: now}}
	b := &Account{UUID: "b", Usage: &Usage{Session: 0, Weekly: 30, CheckedAt: now}}
	s := &switcher{active: "a", st: State{Accounts: []*Account{a, b}, AutoSwitch: true, Threshold: 93, FiveHourOnly: true}}
	if got, why := s.autoTarget(true, now, now); got != b || why != nearlyFull {
		t.Fatal("a used-up week must switch even in five-hour-only mode")
	}
	s.st.AutoSwitch = false
	if got, _ := s.autoTarget(true, now, now); got != nil {
		t.Fatal("the backstop must respect auto-switch being off")
	}
}

func TestKuenHistoryAccounts(t *testing.T) {
	s := &switcher{st: State{Accounts: []*Account{{UUID: "a"}, {UUID: "b"}}}}
	if got := s.historyAccounts(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("enrolled accounts only, got %v", got)
	}
	s.st.SyncAllAccounts = true
	if got := s.historyAccounts(); got != nil {
		t.Fatal("sync-all must not filter")
	}
	if allowedAccount("x", []string{"a"}) || !allowedAccount("a", []string{"a"}) || !allowedAccount("x", nil) {
		t.Fatal("allowedAccount filter is wrong")
	}
}

func TestKuenQuotaView(t *testing.T) {
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)
	snap := snapshot{Language: "zh-CN", Active: "a", AutoSwitch: true, Threshold: 93, Accounts: []Account{
		{UUID: "a", Alias: "Pro A", Usage: &Usage{Session: 42, SessionReset: now.Add(2*time.Hour + 10*time.Minute), Weekly: 11, CheckedAt: now}},
		{UUID: "b", Alias: "Pro B", Usage: &Usage{Session: 0, Weekly: 29, CheckedAt: now}},
	}}
	v := buildQuotaView(snap, now)
	if v.BarTitle != "42%" || v.Pct != 42 {
		t.Fatalf("bar title %q pct %v", v.BarTitle, v.Pct)
	}
	for i, want := range []string{"Pro A", "▰▰▰▰▰▱▱▱▱▱▱▱  42%", "剩余 58%", "本周额度", "自动换到 Pro B", "刚刚"} {
		if !strings.Contains(v.Lines[i], want) {
			t.Fatalf("line %d %q lacks %q", i, v.Lines[i], want)
		}
	}
	if !strings.Contains(v.Lines[2], "还有 2 小时 10 分") {
		t.Fatalf("reset countdown missing: %q", v.Lines[2])
	}
	snap.Active = ""
	if v := buildQuotaView(snap, now); v.BarTitle != "--" || v.Pct != -1 {
		t.Fatal("no active account must show unknown")
	}
}

func TestKuenRestartWarning(t *testing.T) {
	zh := restartWarning("zh-CN", "添加账号", 3*time.Minute)
	if !strings.Contains(zh, "3 分钟前") || !strings.Contains(zh, "添加账号需要重启 Claude") {
		t.Fatalf("zh warning: %q", zh)
	}
	if en := restartWarning("en", "Add account", 0); !strings.Contains(en, "just now") {
		t.Fatalf("en warning: %q", en)
	}
	if got := appleString(`say "hi" \ ok`); got != `"say \"hi\" \\ ok"` {
		t.Fatalf("apple string: %s", got)
	}
}

func TestKuenPanelState(t *testing.T) {
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.Local)
	snap := snapshot{Language: "zh-CN", Active: "a", AutoSwitch: true, Threshold: 93, Accounts: []Account{
		{UUID: "a", Alias: "Pro A", Usage: &Usage{Session: 42, SessionReset: now.Add(2*time.Hour + 10*time.Minute), Weekly: 11, CheckedAt: now}},
		{UUID: "b", Alias: "Pro B", Problem: "signed out, add it again"},
	}}
	st := buildPanelState(snap, true, now)
	if !st.HasActive || st.Pct != 42 || st.Level != "ok" || st.Left != "剩余 58%" || st.Countdown != "2 小时 10 分后重置" {
		t.Fatalf("five-hour fields wrong: %+v", st)
	}
	if !strings.HasPrefix(st.WeekText, "本周额度 11%") || !st.Settings.Autostart || st.Settings.Threshold != 93 {
		t.Fatalf("week or settings wrong: %+v", st)
	}
	if len(st.Accounts) != 2 || !st.Accounts[0].Active || st.Accounts[1].Known || st.Accounts[1].Problem == "" {
		t.Fatalf("accounts wrong: %+v", st.Accounts)
	}
	if level(95, 93) != "full" || level(80, 93) != "warn" || level(-1, 93) != "unknown" {
		t.Fatal("levels wrong")
	}
	if st.Settings.Tint != "grey" || st.Settings.Frost != defaultFrost || len(st.Settings.Tints) != 6 {
		t.Fatalf("glass defaults wrong: %+v", st.Settings)
	}
	s := &switcher{dir: t.TempDir(), onChange: func() {}}
	s.setTint("blue")
	s.setTint("neon")
	s.setFrost(500)
	if s.st.GlassTint != "blue" || s.st.GlassFrost != 100 {
		t.Fatalf("tint or frost not kept in range: %q %d", s.st.GlassTint, s.st.GlassFrost)
	}
	c, err := parsePanelCommand(`{"cmd":"set","key":"auto","on":true}`)
	if err != nil || c.Cmd != "set" || c.Key != "auto" || !c.On {
		t.Fatalf("command parse: %+v %v", c, err)
	}
}
