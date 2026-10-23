// Command relay-for-claude is a menu bar tool for the Claude desktop app that
// switches to the next signed-in account when one runs out of usage.
// It switches the app between signed-in accounts without signing in again,
// keeps Claude Code chats visible in every account and shows plan usage.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"fyne.io/systray"
)

const version = "1.0.0"

// Only one switcher may run; a second start exits quietly.
const instancePort = "127.0.0.1:47823"

func main() {
	status := flag.Bool("status", false, "print accounts and usage, then exit")
	action := flag.String("action", "", "local maintenance: refresh, add, or switch")
	account := flag.Int("account", 0, "1-based saved account number for -action switch")
	flag.Parse()

	// The home folder, because the Store app redirects AppData for everything
	// it starts but leaves the home folder alone.
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(home, ".relay-for-claude")
	// Data from the earlier name moves over once, so saved sign-ins are kept.
	if old := filepath.Join(home, ".claude-account-switcher"); !exists(dir) && exists(old) {
		_ = os.Rename(old, dir)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}
	setupLog(filepath.Join(dir, "switcher.log"), *status)

	if relaunchOutsideDesktop(dir) {
		return
	}

	if data, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
		var saved State
		if json.Unmarshal(data, &saved) != nil {
			fmt.Fprintln(os.Stderr, "Invalid saved state; refusing to overwrite it.")
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Cannot read saved state.")
		os.Exit(1)
	}
	if *status {
		// A status read never polls, saves, restarts Claude or auto-switches,
		// so it does not need the instance lock and works while the tray runs.
		s := newSwitcher(dir)
		s.active, _ = s.app().activeAccount()
		printStatus(s)
		return
	}

	lock, err := net.Listen("tcp", instancePort)
	if err != nil {
		log.Print("already running")
		return
	}
	defer lock.Close()
	if *action != "" {
		s := newSwitcher(dir)
		s.st.AutoSwitch, s.st.SpendWeekly = false, false
		s.st.NoUpdateCheck = true
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		s.refresh(ctx)
		var err error
		switch *action {
		case "refresh":
		case "add":
			err = s.addAccount()
		case "switch":
			if *account < 1 || *account > len(s.st.Accounts) {
				err = fmt.Errorf("invalid saved account number")
			} else {
				err = s.switchTo(s.st.Accounts[*account-1].UUID, byHand)
			}
		default:
			err = fmt.Errorf("unknown maintenance action")
		}
		if err != nil {
			log.Print(err)
			fmt.Fprintln(os.Stderr, "Operation failed; see the private switcher log.")
			os.Exit(1)
		}
		fmt.Println("Operation completed.")
		return
	}

	log.Printf("relay-for-claude %s, app data in %s", version, claudeDesktop().dir)
	t := &tray{s: newSwitcher(dir), updates: make(chan struct{}, 1)}
	systray.Run(t.onReady, func() {})
}

// setupLog writes to switcher.log and starts it over once it passes 1 MB.
// A tray app has no usable stderr on Windows, so only -status also prints there.
func setupLog(path string, console bool) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > 1<<20 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return
	}
	if console {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
	} else {
		log.SetOutput(f)
	}
}

// printStatus reads the saved state without side effects.
func printStatus(s *switcher) {
	snap := s.snapshot()
	if len(snap.Accounts) == 0 {
		fmt.Println("no accounts yet, sign in to Claude")
	}
	for _, a := range snap.Accounts {
		mark := " "
		if a.UUID == snap.Active {
			mark = "*"
		}
		fmt.Printf("%s %s  %s\n", mark, a.label(), usageText(a, snap.Threshold, time.Now()))
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
