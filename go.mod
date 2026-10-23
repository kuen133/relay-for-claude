module github.com/kuen133/relay-for-claude

go 1.26.0

require (
	fyne.io/systray v1.12.2
	golang.org/x/sys v0.48.0
)

require github.com/godbus/dbus/v5 v5.1.0 // indirect

replace fyne.io/systray => ./third_party/systray
