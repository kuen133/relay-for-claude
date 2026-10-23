package main

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// panelState is everything the glass popover shows. The page only draws it;
// all wording that depends on numbers is decided here.
type panelState struct {
	Lang       string         `json:"lang"`
	HasActive  bool           `json:"hasActive"`
	Pct        float64        `json:"pct"` // five-hour usage of the active account, -1 unknown
	Level      string         `json:"level"`
	Left       string         `json:"left"`
	Countdown  string         `json:"countdown"`
	ResetAt    string         `json:"resetAt"`
	Week       float64        `json:"week"`
	WeekText   string         `json:"weekText"`
	Accounts   []panelAccount `json:"accounts"`
	Next       string         `json:"next"`
	NextDetail string         `json:"nextDetail"`
	Status     string         `json:"status"`
	Updated    string         `json:"updated"`
	Busy       bool           `json:"busy"`
	Update     string         `json:"update"`
	Settings   panelSettings  `json:"settings"`
}

type panelAccount struct {
	UUID    string  `json:"uuid"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Active  bool    `json:"active"`
	Five    float64 `json:"five"`
	Week    float64 `json:"week"`
	Known   bool    `json:"known"`
	Problem string  `json:"problem"`
	Level   string  `json:"level"`
}

type panelSettings struct {
	Auto         bool      `json:"auto"`
	Threshold    float64   `json:"threshold"`
	Thresholds   []float64 `json:"thresholds"`
	FiveHourOnly bool      `json:"fiveHourOnly"`
	SpendWeekly  bool      `json:"spendWeekly"`
	SyncAll      bool      `json:"syncAll"`
	Autostart    bool      `json:"autostart"`
	Opacity      int       `json:"opacity"`
	Tint         string    `json:"tint"`
	Tints        []string  `json:"tints"`
	Frost        int       `json:"frost"`
}

// level names the colour of a usage value, matching the tray icon.
func level(pct, threshold float64) string {
	switch {
	case pct < 0:
		return "unknown"
	case pct >= threshold:
		return "full"
	case pct >= warnAt:
		return "warn"
	}
	return "ok"
}

func buildPanelState(snap snapshot, autostart bool, now time.Time) panelState {
	zh := snap.Language == "zh-CN"
	tr := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	st := panelState{Lang: "en", Pct: -1, Level: "unknown", Busy: snap.Busy, Update: snap.Update,
		Settings: panelSettings{Auto: snap.AutoSwitch, Threshold: snap.Threshold, Thresholds: thresholds,
			FiveHourOnly: snap.FiveHourOnly, SpendWeekly: snap.SpendWeekly, SyncAll: snap.SyncAllAccounts, Autostart: autostart, Opacity: snap.GlassOpacity,
			Tint: snap.GlassTint, Tints: glassTints, Frost: snap.GlassFrost}}
	if st.Settings.Tint == "" {
		st.Settings.Tint = "grey"
	}
	if st.Settings.Frost == 0 {
		st.Settings.Frost = defaultFrost
	}
	if st.Settings.Opacity == 0 {
		st.Settings.Opacity = defaultGlass
	}
	if zh {
		st.Lang = "zh-CN"
	}
	switch {
	case snap.Status != "":
		st.Status = translate(snap.Language, snap.Status)
	case snap.Note != "":
		st.Status = snap.Note
	case len(snap.Accounts) == 0:
		st.Status = translate(snap.Language, "Open Claude and sign in")
	}
	var cur *Account
	for i := range snap.Accounts {
		a := &snap.Accounts[i]
		pa := panelAccount{UUID: a.UUID, Name: a.shortName(), Email: a.Email, Active: a.UUID == snap.Active,
			Problem: translate(snap.Language, a.Problem), Level: "unknown"}
		if a.Usage != nil {
			u := a.Usage.current(now)
			pa.Five, pa.Week, pa.Known = u.Session, u.Weekly, true
			pa.Level = level(u.peak(), snap.Threshold)
		}
		if pa.Active {
			cur = a
		}
		st.Accounts = append(st.Accounts, pa)
	}
	if cur == nil {
		return st
	}
	st.HasActive = true
	if cur.Usage == nil {
		st.Left = tr("Usage unknown", "用量未知")
		return st
	}
	u := cur.Usage.current(now)
	st.Pct, st.Week = u.Session, u.Weekly
	st.Level = level(u.Session, snap.Threshold)
	st.Left = fmt.Sprintf(tr("%.0f%% left", "剩余 %.0f%%"), math.Max(100-u.Session, 0))
	if !u.SessionReset.IsZero() && u.SessionReset.After(now) {
		st.Countdown = fmt.Sprintf(tr("Resets in %s", "%s后重置"), durationText(zh, u.SessionReset.Sub(now)))
		st.ResetAt = fmt.Sprintf(tr("Resets at %s", "%s 重置"), localizedReset(snap.Language, u.SessionReset, now))
	} else {
		st.Countdown = tr("Window not started", "五小时窗口未开始")
	}
	st.WeekText = fmt.Sprintf(tr("Weekly %.0f%%", "本周额度 %.0f%%"), u.Weekly)
	if !u.WeeklyReset.IsZero() && u.WeeklyReset.After(now) {
		st.WeekText += fmt.Sprintf(tr(" · resets %s", " · %s 重置"), localizedReset(snap.Language, u.WeeklyReset, now))
	}
	st.NextDetail = nextText(snap, cur, now, tr)
	st.Next = shortNext(st.NextDetail)
	st.Updated = tr("Updated ", "更新于 ") + agoText(zh, now.Sub(cur.Usage.CheckedAt))
	return st
}

// panelCommand is a message the page sends when the user clicks something.
type panelCommand struct {
	Cmd   string  `json:"cmd"`
	UUID  string  `json:"uuid"`
	Key   string  `json:"key"`
	On    bool    `json:"on"`
	Value float64 `json:"value"`
	Lang  string  `json:"lang"`
}

func parsePanelCommand(raw string) (panelCommand, error) {
	var c panelCommand
	err := json.Unmarshal([]byte(raw), &c)
	return c, err
}
