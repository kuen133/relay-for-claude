package main

import (
	"fmt"
	"time"
)

// The OAuth API does not return a subscription's end date (the billing
// details endpoint needs a claude.ai web session, and an Apple purchase is
// renewed by Apple anyway). It does return when the subscription started, and
// a monthly plan renews or ends on that day of every month. So the expiry is
// the next monthly anniversary, unless an exact date was set by hand.

// profileEvery is how often a saved account's profile is read again, to pick
// up a new or cancelled subscription.
const profileEvery = 24 * time.Hour

// expiry returns when the account's current paid month ends, and whether that
// date was set by hand. Zero when there is nothing to go on.
func (a *Account) expiry(now time.Time) (at time.Time, manual bool) {
	if !a.ExpiresAt.IsZero() {
		return a.ExpiresAt, true
	}
	if a.SubSince.IsZero() {
		return time.Time{}, false
	}
	return nextMonthly(a.SubSince, now), false
}

// nextMonthly is the first monthly anniversary of start after now. A start on
// the 31st falls on the last day of shorter months, as App Store billing does.
func nextMonthly(start, now time.Time) time.Time {
	for n := 1; n < 1200; n++ {
		at := addMonthsClamped(start, n)
		if at.After(now) {
			return at
		}
	}
	return time.Time{}
}

func addMonthsClamped(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d, last)-1)
}

// expiryText is the line shown under an account, e.g. "11月21日 07:27 到期 · 还剩 13 天 6 小时".
// Empty when the expiry is unknown.
func expiryText(zh bool, a *Account, now time.Time) string {
	at, manual := a.expiry(now)
	if at.IsZero() {
		return ""
	}
	when := at.Local().Format("Jan 2 15:04")
	if zh {
		when = at.Local().Format("1月2日 15:04")
	}
	mark := ""
	if !manual {
		mark = tr2(zh, " (est.)", "（预计）")
	}
	if !at.After(now) {
		return fmt.Sprintf(tr2(zh, "Expired %s%s", "已于 %s 到期%s"), when, mark)
	}
	return fmt.Sprintf(tr2(zh, "Expires %s%s · %s left", "%s 到期%s · 还剩 %s"), when, mark, daysText(zh, at.Sub(now)))
}

// whenText is a reset or end time in the panel's tidy form: "14:10",
// "明天 14:10", "周三 21:00" within the week, otherwise "11月14日 00:59".
func whenText(zh bool, at, now time.Time) string {
	at, now = at.Local(), now.Local()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	days := int(time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location()).Sub(today).Hours() / 24)
	clock := at.Format("15:04")
	week := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	switch {
	case days == 0:
		return clock
	case days == 1:
		return tr2(zh, "Tomorrow ", "明天 ") + clock
	case days > 1 && days < 7:
		if zh {
			return week[at.Weekday()] + " " + clock
		}
		return at.Format("Mon 15:04")
	case zh:
		return at.Format("1月2日 15:04")
	}
	return at.Format("Jan 2 15:04")
}

// resetParts splits a reset into "14:10 重置" and "还有 2 小时 35 分".
// idle is shown when the window has not started.
func resetParts(zh bool, at, now time.Time, idle string) (when, left string) {
	if at.IsZero() || !at.After(now) {
		return idle, ""
	}
	return fmt.Sprintf(tr2(zh, "%s", "%s 重置"), whenText(zh, at, now)),
		fmt.Sprintf(tr2(zh, "resets in %s", "还有 %s"), daysText(zh, at.Sub(now)))
}

// expiryParts splits the expiry into "11月21日 07:27 到期" and "还剩 13 天 20 小时".
func expiryParts(zh bool, a *Account, now time.Time) (when, left string, estimate bool) {
	at, manual := a.expiry(now)
	if at.IsZero() {
		return "", "", false
	}
	date := at.Local().Format(tr2(zh, "Jan 2 15:04", "1月2日 15:04"))
	if !at.After(now) {
		return fmt.Sprintf(tr2(zh, "Ended %s", "已于 %s 到期"), date), "", !manual
	}
	return fmt.Sprintf(tr2(zh, "Ends %s", "%s 到期"), date),
		fmt.Sprintf(tr2(zh, "%s left", "还剩 %s"), daysText(zh, at.Sub(now))), !manual
}

// daysText is a coarse duration: days and hours, or hours and minutes under a day.
func daysText(zh bool, d time.Duration) string {
	if d < 24*time.Hour {
		return durationText(zh, d)
	}
	days, hours := int(d.Hours())/24, int(d.Hours())%24
	if zh {
		return fmt.Sprintf("%d 天 %d 小时", days, hours)
	}
	return fmt.Sprintf("%dd %dh", days, hours)
}

// expiryLevel colours the expiry: red within a day, orange within three.
func expiryLevel(a *Account, now time.Time) string {
	at, _ := a.expiry(now)
	switch left := at.Sub(now); {
	case at.IsZero():
		return "unknown"
	case left < 24*time.Hour:
		return "full"
	case left < 3*24*time.Hour:
		return "warn"
	}
	return "ok"
}

func tr2(zh bool, en, cn string) string {
	if zh {
		return cn
	}
	return en
}
