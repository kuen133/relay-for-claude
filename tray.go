package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"fyne.io/systray"
)

const maxAccounts = 8

var thresholds = []float64{90, 93, 94, 96, 98}

type tray struct {
	quota      [quotaRows]*systray.MenuItem
	quotaIcon  string
	labels     map[*systray.MenuItem]string
	fiveHour   *systray.MenuItem
	allHistory *systray.MenuItem
	english    *systray.MenuItem
	chinese    *systray.MenuItem
	s          *switcher
	status     *systray.MenuItem
	update     *systray.MenuItem
	accounts   [maxAccounts]*systray.MenuItem
	auto       *systray.MenuItem
	spend      *systray.MenuItem
	thresholds []*systray.MenuItem
	add        *systray.MenuItem
	importNow  *systray.MenuItem
	autostart  *systray.MenuItem
	lastIcon   string
	updates    chan struct{}
}

func (t *tray) onReady() {
	t.labels = map[*systray.MenuItem]string{}
	systray.SetTooltip("Relay for Claude")
	t.setIcon(-1, defaultThreshold)

	// The five-hour quota comes first: it is what the menu is opened for.
	for i := range t.quota {
		t.quota[i] = systray.AddMenuItem("", "")
		t.quota[i].Hide()
	}
	systray.AddSeparator()

	t.status = systray.AddMenuItem("", "")
	t.status.Disable()
	t.status.Hide()
	t.update = systray.AddMenuItem("", "Open the download page")
	t.update.Hide()
	for i := range t.accounts {
		t.accounts[i] = systray.AddMenuItemCheckbox("", "Switch the Claude app to this account", false)
		t.accounts[i].Hide()
	}
	systray.AddSeparator()
	// The top level is for reading quota and picking an account; everything
	// else lives under Settings, with its own submenus a level further down.
	settings := systray.AddMenuItem("Settings", "")
	t.labels[settings] = "Settings"
	sub := func(title, tip string, check bool) *systray.MenuItem {
		var item *systray.MenuItem
		if check {
			item = settings.AddSubMenuItemCheckbox(title, tip, false)
		} else {
			item = settings.AddSubMenuItem(title, tip)
		}
		t.labels[item] = title
		return item
	}
	t.auto = settings.AddSubMenuItemCheckbox("Auto-switch when nearly full", "Switch to the account with the most room left", false)
	t.labels[t.auto] = "Auto-switch when nearly full"
	limit := sub("Auto-switch at", "", false)
	for _, v := range thresholds {
		t.thresholds = append(t.thresholds, limit.AddSubMenuItemCheckbox(fmt.Sprintf("%.0f%%", v), "", false))
	}
	t.fiveHour = sub("Use 5-hour limit only", "", true)
	t.spend = sub("Auto-switch to use up expiring weekly", "Switch to an account whose weekly limit resets within a day with half or more of it unused", true)
	settings.AddSeparator()
	t.add = sub("Add account…", "Restart Claude on its sign-in screen, your current account is kept", false)
	t.importNow = sub("Import chats from all accounts", "Restart Claude so every chat shows up in this account", false)
	t.allHistory = sub("Sync history from all accounts", "", true)
	settings.AddSeparator()
	lang := settings.AddSubMenuItem("语言 / Language", "")
	t.chinese = lang.AddSubMenuItemCheckbox("简体中文", "", false)
	t.english = lang.AddSubMenuItemCheckbox("English", "", false)
	// Rewriting an enabled entry keeps its path current and adds the Task
	// Manager record that entries from before v0.1.2 are missing.
	if autostartEnabled() {
		if err := setAutostart(true); err != nil {
			log.Printf("refresh start at login: %v", err)
		}
	}
	t.autostart = settings.AddSubMenuItemCheckbox("Start at login", "", autostartEnabled())
	t.labels[t.autostart] = "Start at login"
	folder := sub("Open data folder", "", false)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "")
	t.labels[quit] = "Quit"

	for i, item := range t.accounts {
		go func() {
			for range item.ClickedCh {
				snap := t.s.snapshot()
				if i < len(snap.Accounts) && snap.Accounts[i].UUID != snap.Active {
					go t.s.run("Switch", func() error { return t.s.switchTo(snap.Accounts[i].UUID, byHand) })
				}
				t.s.onChange()
			}
		}()
	}
	for i, item := range t.thresholds {
		go func() {
			for range item.ClickedCh {
				t.s.setThreshold(thresholds[i])
			}
		}()
	}
	go func() {
		for {
			select {
			case <-t.auto.ClickedCh:
				t.s.setAuto(!t.auto.Checked())
			case <-t.spend.ClickedCh:
				t.s.setSpendWeekly(!t.spend.Checked())
			case <-t.fiveHour.ClickedCh:
				t.s.setFlag(&t.s.st.FiveHourOnly, !t.fiveHour.Checked())
			case <-t.allHistory.ClickedCh:
				t.s.setFlag(&t.s.st.SyncAllAccounts, !t.allHistory.Checked())
			case <-t.chinese.ClickedCh:
				t.setLanguage("zh-CN")
			case <-t.english.ClickedCh:
				t.setLanguage("en")
			case <-t.add.ClickedCh:
				t.guardedRun("Add account", t.s.addAccount)
			case <-t.importNow.ClickedCh:
				t.guardedRun("Import", t.s.importChats)
			case <-t.autostart.ClickedCh:
				if err := setAutostart(!t.autostart.Checked()); err != nil {
					t.s.setStatus("Start at login: " + err.Error())
				}
				setChecked(t.autostart, autostartEnabled())
			case <-t.update.ClickedCh:
				shellOpen(repoURL + "/releases/latest")
			case <-folder.ClickedCh:
				shellOpen(t.s.dir)
			case <-quit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Menu updates arrive from worker goroutines; one goroutine applies them.
	t.s.onChange = func() {
		select {
		case t.updates <- struct{}{}:
		default:
		}
	}
	go func() {
		for range t.updates {
			t.render()
		}
	}()
	t.s.onChange()
	if hasGlassPanel {
		// A left click opens the glass panel; a right click keeps the menu.
		glassInit(t.panelCommand)
		systray.SetOnTapped(glassToggle)
		// RELAY_PANEL_PREVIEW opens the panel at start, for screenshots.
		if os.Getenv("RELAY_PANEL_PREVIEW") != "" {
			time.AfterFunc(3*time.Second, glassToggle)
		}
	}
	go t.loop()
	go t.updateLoop()
}

// loop runs refresh every few seconds while a new sign-in is expected,
// otherwise every minute; refresh decides which accounts are due.
func (t *tray) loop() {
	var last time.Time
	for ; ; time.Sleep(5 * time.Second) {
		if !t.s.adding() && time.Since(last) < hotEvery {
			continue
		}
		last = time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		t.s.refresh(ctx)
		cancel()
	}
}

// updateLoop looks for a new release a minute after start, then every hour;
// checkUpdate decides whether a day has passed since the last answer.
func (t *tray) updateLoop() {
	time.Sleep(time.Minute)
	for {
		t.s.checkUpdate(context.Background())
		time.Sleep(time.Hour)
	}
}

func (t *tray) render() {
	snap := t.s.snapshot()
	for item, label := range t.labels {
		item.SetTitle(translate(snap.Language, label))
	}
	setChecked(t.fiveHour, snap.FiveHourOnly)
	setChecked(t.allHistory, snap.SyncAllAccounts)
	setChecked(t.chinese, snap.Language == "zh-CN")
	setChecked(t.english, snap.Language != "zh-CN")
	now := time.Now()

	switch {
	case snap.Status != "":
		t.status.SetTitle(translate(snap.Language, snap.Status))
		t.status.Show()
	case snap.Note != "":
		t.status.SetTitle(snap.Note)
		t.status.Show()
	default:
		t.status.Hide()
	}
	if snap.Update != "" {
		t.update.SetTitle("Version " + snap.Update + " is out, click to download")
		t.update.Show()
	} else {
		t.update.Hide()
	}
	if len(snap.Accounts) == 0 && snap.Status == "" {
		t.status.SetTitle(translate(snap.Language, "Open Claude and sign in"))
		t.status.Show()
	}

	activePct := -1.0
	for i, item := range t.accounts {
		if i >= len(snap.Accounts) {
			item.Hide()
			continue
		}
		a := snap.Accounts[i]
		accountTitle := a.label()
		if email := strings.TrimSpace(a.Email); email != "" && !strings.Contains(accountTitle, email) {
			accountTitle += " · " + email
		}
		detail := localizedUsage(snap.Language, usageText(a, snap.Threshold, now))
		item.SetTitle(accountTitle + menuTab + detail)
		setChecked(item, a.UUID == snap.Active)
		if snap.Busy {
			item.Disable()
		} else {
			item.Enable()
		}
		item.Show()
		if a.UUID == snap.Active {
			if a.Usage != nil {
				activePct = a.Usage.current(now).Session
			}
		}
	}
	view := buildQuotaView(snap, now)
	var tipLines []string
	for i, item := range t.quota {
		if view.Lines[i] == "" {
			item.Hide()
			continue
		}
		item.SetTitle(view.Lines[i])
		item.Show()
		tipLines = append(tipLines, view.Lines[i])
	}
	if key := fmt.Sprintf("%.0f/%.0f", view.Pct, snap.Threshold); runtime.GOOS == "darwin" && key != t.quotaIcon {
		t.quotaIcon = key
		t.quota[1].SetIcon(pngIcon(drawIcon(view.Pct, snap.Threshold)))
	}
	systray.SetTooltip(strings.Join(tipLines, "\n"))
	systray.SetTitle(view.BarTitle)
	t.setIcon(activePct, snap.Threshold)
	if hasGlassPanel {
		glassUpdate(buildPanelState(snap, autostartEnabled(), now))
	}

	setChecked(t.auto, snap.AutoSwitch)
	setChecked(t.spend, snap.SpendWeekly)
	for i, item := range t.thresholds {
		setChecked(item, thresholds[i] == snap.Threshold)
	}
	for _, item := range []*systray.MenuItem{t.add, t.importNow} {
		if snap.Busy {
			item.Disable()
		} else {
			item.Enable()
		}
	}
}

func (t *tray) setLanguage(lang string) {
	t.s.mu.Lock()
	t.s.st.Language = lang
	t.s.saveState()
	t.s.mu.Unlock()
	t.s.onChange()
}

// menuTab right-aligns the usage on Windows, where a tab in a menu title
// starts a right-aligned column.
var menuTab = map[bool]string{true: "\t", false: "    "}[runtime.GOOS == "windows"]

func usageText(a Account, threshold float64, now time.Time) string {
	if a.Problem != "" {
		return a.Problem
	}
	if a.Usage == nil {
		return "usage unknown"
	}
	u := a.Usage.current(now)
	parts := []string{fmt.Sprintf("5h %.0f%%", u.Session), fmt.Sprintf("week %.0f%%", u.Weekly)}
	if u.Session >= threshold && !u.SessionReset.IsZero() {
		parts[0] += " until " + resetText(u.SessionReset, now)
	}
	// The weekly reset is always shown: it decides how much of the week is left to use.
	switch {
	case u.WeeklyReset.IsZero() || !u.WeeklyReset.After(now):
	case u.Weekly >= threshold:
		parts[1] += " until " + resetText(u.WeeklyReset, now)
	default:
		parts[1] += ", resets " + resetText(u.WeeklyReset, now)
	}
	return strings.Join(parts, " · ")
}

func resetText(at, now time.Time) string {
	at, now = at.Local(), now.Local()
	switch {
	case at.YearDay() == now.YearDay() && at.Year() == now.Year():
		return at.Format("15:04")
	case at.Sub(now) < 6*24*time.Hour:
		return at.Format("Mon 15:04")
	}
	// A weekday alone would read as today for a reset a week out.
	return at.Format("Mon 2 Jan 15:04")
}

func (t *tray) setIcon(pct, threshold float64) {
	key := fmt.Sprintf("%.0f/%.0f", pct, threshold)
	if key == t.lastIcon {
		return
	}
	t.lastIcon = key
	systray.SetIcon(trayIcon(pct, threshold))
}

func setChecked(item *systray.MenuItem, on bool) {
	if on != item.Checked() {
		if on {
			item.Check()
		} else {
			item.Uncheck()
		}
	}
}

// panelCommand carries out a click in the glass panel.
func (t *tray) panelCommand(c panelCommand) {
	switch c.Cmd {
	case "ready":
		t.s.onChange()
	case "switch":
		snap := t.s.snapshot()
		if c.UUID != "" && c.UUID != snap.Active && !snap.Busy {
			go t.s.run("Switch", func() error { return t.s.switchTo(c.UUID, byHand) })
		}
	case "set":
		switch c.Key {
		case "auto":
			t.s.setAuto(c.On)
		case "fiveHourOnly":
			t.s.setFlag(&t.s.st.FiveHourOnly, c.On)
		case "spendWeekly":
			t.s.setSpendWeekly(c.On)
		case "syncAll":
			t.s.setFlag(&t.s.st.SyncAllAccounts, c.On)
		case "autostart":
			if err := setAutostart(c.On); err != nil {
				t.s.setStatus("Start at login: " + err.Error())
			}
			setChecked(t.autostart, autostartEnabled())
			t.s.onChange()
		}
	case "threshold":
		for _, v := range thresholds {
			if v == c.Value {
				t.s.setThreshold(v)
			}
		}
	case "opacity":
		t.s.setGlass(int(c.Value))
	case "tint":
		t.s.setTint(c.Key)
	case "frost":
		t.s.setFrost(int(c.Value))
	case "lang":
		if c.Lang == "zh-CN" || c.Lang == "en" {
			t.setLanguage(c.Lang)
		}
	case "add":
		glassHide()
		t.guardedRun("Add account", t.s.addAccount)
	case "import":
		glassHide()
		t.guardedRun("Import", t.s.importChats)
	case "folder":
		glassHide()
		shellOpen(t.s.dir)
	case "update":
		shellOpen(repoURL + "/releases/latest")
	case "quit":
		systray.Quit()
	}
}
