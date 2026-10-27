/*
	MiracleOS's desktop - A taskbar, with a main menu and a notification daemon
    Copyright (C) 2025 MiracleOS Contributors

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.

*/

package main

import (
	"context"
	"github.com/MiracleOS-Team/desktop/internal/apptabs"
	_ "embed"
	"log"
	"os"
	"runtime"

	"github.com/MiracleOS-Team/libxdg-go/desktopFiles"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

//go:embed desktop.css
var cssData string

func loadCSS() {
	// Load CSS into GTK
	provider, _ := gtk.CssProviderNew()
	//err = provider.LoadFromData(css)

	err := provider.LoadFromData(cssData)
	if err != nil {

		log.Println("Failed to load CSS into GTK:", err)
		return

	}

	display, err := gdk.DisplayGetDefault()
	if err != nil {
		log.Println("Failed to get default display:", err)
		return
	}

	screen, err := display.GetDefaultScreen()
	if err != nil {
		log.Println("Failed to get default screen:", err)
		return
	}

	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

// scalePixbuf scales a Pixbuf while maintaining aspect ratio
func scalePixbuf(pixbuf *gdk.Pixbuf, maxWidth, maxHeight int) *gdk.Pixbuf {
	width := pixbuf.GetWidth()
	height := pixbuf.GetHeight()

	// Maintain aspect ratio
	scaleFactor := float64(maxWidth) / float64(width)
	if float64(maxHeight)/float64(height) < scaleFactor {
		scaleFactor = float64(maxHeight) / float64(height)
	}

	newWidth := int(float64(width) * scaleFactor)
	newHeight := int(float64(height) * scaleFactor)

	scaledPixbuf, err := pixbuf.ScaleSimple(newWidth, newHeight, gdk.INTERP_BILINEAR)
	if err != nil {
		log.Fatal("Could not scale image:", err)
	}
	return scaledPixbuf
}

func firstN(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}

func main() {
	if len(os.Args)>1 && os.Args[1]=="--app-tabs" { if err:=apptabs.Run(context.Background());err!=nil{log.Fatal(err)};return }
	runtime.LockOSThread()
	if len(os.Args) > 1 && os.Args[1] == "--login" {
		gtk.Init(&os.Args)
		showLoginScreen()
		return
	}
	if err := ensureTerminalLauncher(); err != nil {
		log.Print(err)
	}
	if err := ensurePackageInstallerLauncher(); err != nil {
		log.Print(err)
	}
	if err := ensureWindowsLauncher(); err != nil { log.Print(err) }
	if len(os.Args) > 1 && os.Args[1] == "--context-menu" {
		runSharedContextMenu()
		return
	}
	os.Setenv("GTK_CSD", "1")
	if len(os.Args) > 1 && os.Args[1] == "--settings" {
		os.Args[0] = "miracleos-settings"
	}
	if len(os.Args) > 1 && os.Args[1] == "--files" {
		os.Args[0] = "miracleos-files"
	}
	if len(os.Args) > 1 && os.Args[1] == "--install" {
		os.Args[0] = "miracleos-install"
	}
	if len(os.Args)>1 && os.Args[1]=="--windows" { os.Args[0]="miracleos-windows" }
	gtk.Init(&os.Args)
	loadCSS()
	watchSystemTheme()
	if len(os.Args)>1 && os.Args[1]=="--windows" {
		path:="";if len(os.Args)>2{path=os.Args[2]};showWindowsApp(path);gtk.Main();return
	}
 if len(os.Args)>1&&os.Args[1]=="--spotify-overlay"{showSpotifyOverlay();return}
 if len(os.Args)>1&&(os.Args[1]=="--setup"||os.Args[1]=="--setup-preview"){showSystemSetup(os.Args[1]=="--setup-preview");gtk.Main();return}
 if len(os.Args)>1&&os.Args[1]=="--install-system"{showSystemInstaller();gtk.Main();return}
	if len(os.Args) > 1 && os.Args[1] == "--terminal" {
		showTerminal()
		gtk.Main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--portal" {
		runFilePortal()
		gtk.Main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--settings" {
		showSettings()
		gtk.Main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--startup-preview" {
		showStartupScreen(true)
		gtk.Main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--install" {
		path := ""
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		showPackageInstaller(path)
		gtk.Main()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--files" {
		path := ""
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		showFileManager(path)
		gtk.Main()
		return
	}

	if systemSetupPending() { showSystemSetup(false); gtk.Main(); return }
	apps1, _ := desktopFiles.ListAllApplications()

	bar := createBar(apps1)
	bar.ShowAll()
	createDesktopBackground()
	if len(os.Args) > 1 && os.Args[1] == "--welcome" {
		showStartupScreen(false)
	}

	gtk.Main()
}
