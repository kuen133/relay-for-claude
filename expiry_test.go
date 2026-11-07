package main

import (
	"strings"
	"testing"
	"time"
)

func TestNextMonthly(t *testing.T) {
	start := time.Date(2026, 10, 20, 23, 27, 0, 0, time.UTC)
	now := time.Date(2026, 11, 7, 1, 0, 0, 0, time.UTC)
	if got := nextMonthly(start, now); !got.Equal(time.Date(2026, 11, 20, 23, 27, 0, 0, time.UTC)) {
		t.Fatalf("got %v", got)
	}
	// Past this month's renewal, the next one is a month later.
	now = time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	if got := nextMonthly(start, now); !got.Equal(time.Date(2026, 12, 20, 23, 27, 0, 0, time.UTC)) {
		t.Fatalf("got %v", got)
	}
	// A start on the 31st falls on the last day of shorter months.
	start = time.Date(2026, 1, 31, 8, 0, 0, 0, time.UTC)
	now = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if got := nextMonthly(start, now); !got.Equal(time.Date(2026, 2, 28, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %v", got)
	}
}

func TestExpiryText(t *testing.T) {
	now := time.Date(2026, 11, 7, 1, 0, 0, 0, time.UTC)
	a := &Account{SubSince: time.Date(2026, 10, 20, 23, 27, 0, 0, time.UTC)}
	got := expiryText(true, a, now)
	if !strings.Contains(got, "到期（预计）") || !strings.Contains(got, "还剩 13 天 22 小时") {
		t.Fatalf("got %q", got)
	}
	if expiryLevel(a, now) != "ok" {
		t.Fatal("13 days left must be ok")
	}
	// A hand-set date wins and carries no estimate mark.
	a.ExpiresAt = now.Add(30 * time.Hour)
	got = expiryText(true, a, now)
	if strings.Contains(got, "预计") || !strings.Contains(got, "还剩 1 天 6 小时") || expiryLevel(a, now) != "warn" {
		t.Fatalf("got %q %s", got, expiryLevel(a, now))
	}
	a.ExpiresAt = now.Add(-time.Hour)
	if got = expiryText(true, a, now); !strings.HasPrefix(got, "已于") {
		t.Fatalf("got %q", got)
	}
	if expiryText(true, &Account{}, now) != "" {
		t.Fatal("unknown expiry must be empty")
	}
}

func TestApplyProfile(t *testing.T) {
	var p profile
	p.Organization.Type = "claude_pro"
	p.Organization.SubSince = "2026-10-20T23:27:25.853324Z"
	p.Organization.BillingType = "apple_subscription"
	a := &Account{}
	now := time.Now()
	a.applyProfile(p, now)
	if a.Plan != "Pro" || a.SubSince.IsZero() || a.Billing != "apple_subscription" || !a.ProfileAt.Equal(now) {
		t.Fatalf("got %+v", a)
	}
}

func TestFiveResetText(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 34, 0, 0, time.Local)
	u := Usage{Session: 40, SessionReset: now.Add(2*time.Hour + 35*time.Minute)}
	if got, want := fiveResetText("zh-CN", u, now), "五小时 13:09 重置 · 还有 2 小时 35 分"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := fiveResetText("zh-CN", Usage{}, now); got != "五小时窗口未开始" {
		t.Fatalf("got %q", got)
	}
	a := Account{Usage: &u}
	if got, want := usageText(a, 93, now), "5h 40%, resets 13:09 · week 0%"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := localizedUsage("zh-CN", usageText(a, 93, now)), "五小时 40%，重置于 13:09 · 本周 0%"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResetParts(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	at, in := resetParts(true, now.Add(4*24*time.Hour+10*time.Hour), now, "未开始")
	if at != "周三 20:00 重置" || in != "还有 4 天 10 小时" {
		t.Fatalf("got %q %q", at, in)
	}
	at, in = resetParts(true, now.Add(21*time.Hour), now, "未开始")
	if at != "明天 07:00 重置" || in != "还有 21 小时 0 分" {
		t.Fatalf("got %q %q", at, in)
	}
	if at, in = resetParts(true, time.Time{}, now, "未开始"); at != "未开始" || in != "" {
		t.Fatalf("got %q %q", at, in)
	}
	if got := whenText(true, now.AddDate(0, 0, 9), now); got != "11月16日 10:00" {
		t.Fatalf("got %q", got)
	}
}

func TestWeekTotal(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	snap := snapshot{Language: "zh-CN", Threshold: 93, Accounts: []Account{
		{Alias: "A", Usage: &Usage{Weekly: 26, WeeklyReset: now.Add(4 * 24 * time.Hour)}},
		{Alias: "B", Usage: &Usage{Weekly: 13, WeeklyReset: now.Add(6 * 24 * time.Hour)}},
		{Alias: "C", Usage: &Usage{Weekly: 37, WeeklyReset: now.Add(21 * time.Hour)}},
		{Alias: "D", Plan: "Pro", Problem: "signed out, add it again"},
		{Alias: "E"},
	}}
	for i := 0; i < 3; i++ {
		snap.Accounts[i].Plan = "Pro"
	}
	w := buildWeekTotal(snap, now)
	if w.Known != 3 || w.All != 4 || int(w.Used*100) != 2533 || len(w.Segs) != 5 || w.Segs[3] != -1 || w.Levels[4] != "none" {
		t.Fatalf("got %+v", w)
	}
	if w.Left != "剩余 75% · 1 个未读到" || w.Next != "最早 C 明天 07:00 重置" {
		t.Fatalf("got %q %q", w.Left, w.Next)
	}
	// A week that has reset counts as unused.
	snap.Accounts[2].Usage.WeeklyReset = now.Add(-time.Hour)
	if w = buildWeekTotal(snap, now); w.Segs[2] != 0 {
		t.Fatalf("got %+v", w.Segs)
	}
}
