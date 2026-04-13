package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"plexcovermanager/appversion"
	"plexcovermanager/config"
	"plexcovermanager/diagnostics"
	"plexcovermanager/ui"
)

func main() {
	diagnostics.Log("main: start version=%s", appversion.Version)
	if os.Getenv("PCM_SKIP_APP_OPENGL_CHECK") != "1" {
		if err := diagnostics.CheckOpenGL(); err != nil {
			diagnostics.Log("main: opengl preflight failed: %v", err)
			diagnostics.ShowGraphicsError(err)
			return
		}
		diagnostics.Log("main: opengl preflight ok")
	} else {
		diagnostics.Log("main: opengl preflight skipped by launcher")
	}
	configManager, err := config.NewManager()
	if err != nil {
		diagnostics.Log("main: config error: %v", err)
		showStartupError(err)
		return
	}
	diagnostics.Log("main: config loaded from %s", configManager.Path())
	ui.NewApplication(configManager).Run()
	diagnostics.Log("main: exit")
}

func showStartupError(err error) {
	log.Printf("startup error: %v", err)
	app := fyneapp.NewWithID("de.plexcovermanager.startup")
	window := app.NewWindow(appversion.DisplayName())
	window.Resize(fyne.NewSize(520, 180))
	dialog.ShowError(err, window)
	window.ShowAndRun()
}
