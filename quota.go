package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// quotaView is the five-hour-first summary shown in the status bar and at the
// top of the menu, in the spirit of TokenStep's usage panel.
type quotaView struct {
	// BarTitle sits beside the ring in the status bar.
	BarTitle string
	// Pct is the active account's five-hour usage, -1 when unknown.
	Pct float64
	// Lines are the menu header rows, top to bottom.
	Lines [quotaRows]string
}

const (
	quotaRows = 6
	barCells  = 12
)

func buildQuotaView(snap snapshot, now time.Time) quotaView {
	zh := snap.Language == "zh-CN"
	tr := func(en, cn string) string {
		if zh {
			return cn
		}
		return en
	}
	v := quotaView{BarTitle: "--", Pct: -1}
	var cur *Account
	for i := range snap.Accounts {
		if snap.Accounts[i].UUID == snap.Active {
			cur = &snap.Accounts[i]
		}
	}
	if cur == nil {
		v.Lines[0] = tr("No active Claude account", "当前没有登录的 Claude 账号")
		return v
	}
	v.Lines[0] = tr("Active account  ", "当前账号  ") + cur.label()
	switch {
	case cur.Problem != "":
		v.Lines[1] = tr("5-hour limit  ", "五小时额度  ") + translate(snap.Language, cur.Problem)
		return v
	case cur.Usage == nil:
		v.Lines[1] = tr("5-hour limit  usage unknown", "五小时额度  用量未知")
		return v
	}
	u := cur.Usage.current(now)
	v.Pct = u.Session
	v.BarTitle = fmt.Sprintf("%.0f%%", u.Session)

	v.Lines[1] = fmt.Sprintf("%s%s  %.0f%%", tr("5-hour limit  ", "五小时额度  "), usageBar(u.Session), u.Session)
	left := fmt.Sprintf(tr("%.0f%% left", "剩余 %.0f%%"), math.Max(100-u.Session, 0))
	if !u.SessionReset.IsZero() && u.SessionReset.After(now) {
		left += fmt.Sprintf(tr(" · resets %s (in %s)", " · %s 重置（还有 %s）"),
			localizedReset(snap.Language, u.SessionReset, now), durationText(zh, u.SessionReset.Sub(now)))
	}
	v.Lines[2] = left

	week := fmt.Sprintf("%s%s  %.0f%%", tr("Weekly limit  ", "本周额度    "), usageBar(u.Weekly), u.Weekly)
	if !u.WeeklyReset.IsZero() && u.WeeklyReset.After(now) {
		week += fmt.Sprintf(tr(" · resets %s", " · %s 重置"), localizedReset(snap.Language, u.WeeklyReset, now))
	}
	v.Lines[3] = week
	v.Lines[4] = nextText(snap, cur, now, tr)
	v.Lines[5] = tr("Updated ", "更新于 ") + agoText(zh, now.Sub(cur.Usage.CheckedAt))
	return v
}

// nextText says what auto-switch will do and where it would go.
func nextText(snap snapshot, cur *Account, now time.Time, tr func(string, string) string) string {
	if !snap.AutoSwitch {
		return tr("Auto-switch is off", "自动切换已关闭")
	}
	var best *Account
	for i := range snap.Accounts {
		a := &snap.Accounts[i]
		if a.UUID == cur.UUID || a.Usage == nil || a.Problem != "" {
			continue
		}
		if best == nil || a.Usage.current(now).peak() < best.Usage.current(now).peak() {
			best = a
		}
	}
	at := fmt.Sprintf("%.0f%%", snap.Threshold)
	if best == nil || best.Usage.current(now).peak() >= snap.Threshold-autoHeadroom {
		return fmt.Sprintf(tr("At %s: no other account has room", "用到 %s 时：暂无额度充足的备用账号"), at)
	}
	b := best.Usage.current(now)
	return fmt.Sprintf(tr("At %s switches to %s (5h %.0f%% · week %.0f%%)", "用到 %s 自动换到 %s（五小时 %.0f%% · 本周 %.0f%%）"),
		at, best.label(), b.Session, b.Weekly)
}

// shortName is an account's alias or name without the plan, for the cards.
func (a *Account) shortName() string {
	switch {
	case a.Alias != "":
		return a.Alias
	case a.Name != "":
		return a.Name
	}
	return a.Email
}

// shortNext is nextText cut before its usage details, for the panel's pill.
func shortNext(s string) string {
	for _, sep := range []string{"（", " ("} {
		if i := strings.Index(s, sep); i > 0 {
			return s[:i]
		}
	}
	return s
}

func usageBar(pct float64) string {
	n := int(math.Round(math.Min(math.Max(pct, 0), 100) / 100 * barCells))
	return strings.Repeat("▰", n) + strings.Repeat("▱", barCells-n)
}

func durationText(zh bool, d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case zh && h > 0:
		return fmt.Sprintf("%d 小时 %d 分", h, m)
	case zh:
		return fmt.Sprintf("%d 分钟", m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func agoText(zh bool, d time.Duration) string {
	if d < time.Minute {
		if zh {
			return "刚刚"
		}
		return "just now"
	}
	if zh {
		return durationText(zh, d) + "前"
	}
	return durationText(zh, d) + " ago"
}

// localizedReset is resetText with Chinese weekday names.
func localizedReset(lang string, at, now time.Time) string {
	s := resetText(at, now)
	if lang != "zh-CN" {
		return s
	}
	return strings.NewReplacer("Mon", "周一", "Tue", "周二", "Wed", "周三", "Thu", "周四",
		"Fri", "周五", "Sat", "周六", "Sun", "周日").Replace(s)
}
