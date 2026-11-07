package main

import (
	"fmt"
	"time"
)

// weekTotal is the weekly limit of all subscribed accounts taken together.
// Every account counts as one equal share, which holds while they are all on
// the same plan.
type weekTotal struct {
	// Used is the share of the combined weekly allowance used, 0-100.
	Used  float64 `json:"used"`
	Known int     `json:"known"` // accounts with a reading
	All   int     `json:"all"`
	// Segs is each account's weekly usage in panel order, -1 when unknown.
	Segs   []float64 `json:"segs"`
	Levels []string  `json:"levels"`
	Left   string    `json:"left"` // "剩余 75%"
	Next   string    `json:"next"` // "最早 Pro C 周四 07:59 重置"
}

func buildWeekTotal(snap snapshot, now time.Time) weekTotal {
	zh := snap.Language == "zh-CN"
	var w weekTotal
	var sum float64
	var first *Account
	var firstAt time.Time
	for i := range snap.Accounts {
		a := &snap.Accounts[i]
		if a.Plan == "" {
			// Not subscribed: no weekly allowance to add up.
			w.Segs, w.Levels = append(w.Segs, -1), append(w.Levels, "none")
			continue
		}
		w.All++
		if a.Usage == nil || a.Problem != "" {
			w.Segs, w.Levels = append(w.Segs, -1), append(w.Levels, "unknown")
			continue
		}
		u := a.Usage.current(now)
		w.Known++
		sum += u.Weekly
		w.Segs = append(w.Segs, u.Weekly)
		w.Levels = append(w.Levels, level(u.Weekly, snap.Threshold))
		if at := u.WeeklyReset; at.After(now) && (first == nil || at.Before(firstAt)) {
			first, firstAt = a, at
		}
	}
	if w.Known == 0 {
		w.Left = tr2(zh, "Usage unknown", "用量未知")
		return w
	}
	w.Used = sum / float64(w.Known)
	w.Left = fmt.Sprintf(tr2(zh, "%.0f%% left", "剩余 %.0f%%"), 100-w.Used)
	if w.Known < w.All {
		w.Left += fmt.Sprintf(tr2(zh, " · %d unread", " · %d 个未读到"), w.All-w.Known)
	}
	if first != nil {
		w.Next = fmt.Sprintf(tr2(zh, "%s resets first, %s", "最早 %s %s 重置"), first.shortName(), whenText(zh, firstAt, now))
	}
	return w
}
