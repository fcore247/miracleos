package main

import (
	"github.com/MiracleOS-Team/desktop/internal/apptabs"
	"strconv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type filePlace struct{ name, folder, icon string }

var filePlaces = []filePlace{
	{"Dokumente", "Documents", "text-x-generic-symbolic"},
	{"Downloads", "Downloads", "folder-download-symbolic"},
	{"Bilder", "Pictures", "image-x-generic-symbolic"},
	{"Musik", "Music", "audio-x-generic-symbolic"},
	{"Videos", "Videos", "video-x-generic-symbolic"},
}

type glassFileManager struct {
	frameTabs *apptabs.Client
	nextFrameTab int
	overlay              *gtk.Overlay
	dragOverview         gtk.IWidget
	dragOverviewStop     func()
	tagDots              map[*gtk.Label]string
	tabsBar              *gtk.Box
	tabs                 []explorerTab
	activeTab            int
	window               *gtk.Window
	home, current        string
	history              []string
	historyAt            int
	path, search         *gtk.Entry
	title, status        *gtk.Label
	content              *gtk.Box
	view                 gtk.IWidget
	back, forward        *gtk.Button
	hidden               bool
	favorites            []string
	sidebar              map[string]*gtk.Button
	picker               *filePickerOptions
	pickerSelection      []string
	selection            []string
	pickerName           *gtk.Entry
	gallery              bool
	reverseSort, compact bool
	generation           uint64
	alive                atomic.Bool
}

var thumbnailSlots = make(chan struct{}, 4)

func launchFileManager(path string) {
	hideStartMenu()
	binary, err := os.Executable()
	if err == nil {
		exec.Command(binary, "--files", path).Start()
	}
}

func showFileManager(path string) {
	newGlassFileManager(path, nil)
}

func newGlassFileManager(path string, picker *filePickerOptions) *glassFileManager {
	home, _ := os.UserHomeDir()
	fm := &glassFileManager{home: home, historyAt: -1, sidebar: map[string]*gtk.Button{}}
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	fm.window = win
	fm.alive.Store(true)
	win.Connect("destroy", func() { fm.alive.Store(false); atomic.AddUint64(&fm.generation, 1) })
	fm.picker = picker
	win.SetTitle("Dateien — MiracleOS")
	win.SetDefaultSize(1120, 650)
	windowTitle := "Dateien — MiracleOS"
	if picker != nil {
		windowTitle = "Dateiauswahl — MiracleOS"
		win.SetTitle(windowTitle)
		if picker.Parent != nil {
			win.SetTransientFor(picker.Parent)
			win.SetModal(true)
			win.SetDestroyWithParent(true)
		}
		win.Connect("destroy", func() { picker.finish(nil) })
	} else {
		win.Connect("destroy", gtk.MainQuit)
	}
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	menuClass(root, "fm-explorer")
	root.PackStart(glassWindowHeader(win, windowTitle), false, false, 0)
	if picker == nil {
		fm.frameTabs, _ = apptabs.Connect(func(a apptabs.Action) {
			glib.IdleAdd(func() { if !fm.alive.Load(){return}; switch a.Action {
			case "new": fm.openTab(fm.home)
			case "select", "close": id,e:=strconv.Atoi(a.ID);if e==nil{for i,t:=range fm.tabs{if t.frameID==id{if a.Action=="select"{fm.saveTab();fm.selectTab(i)}else{fm.closeTab(i)};break}}}
			} })
		})
		win.Connect("destroy",func(){fm.frameTabs.Close()})
		fm.tabsBar, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
		menuClass(fm.tabsBar, "fm-tabs")
		tabScroll, _ := gtk.ScrolledWindowNew(nil, nil)
		tabScroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_NEVER)
		tabScroll.Add(fm.tabsBar)
		if fm.frameTabs==nil { root.PackStart(tabScroll, false, false, 0) }
	}
	toolbar, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 7)
	menuClass(toolbar, "fm-toolbar")
	fm.back = fm.control("‹", "Zurück", func() { fm.travel(-1) })
	fm.forward = fm.control("›", "Vorwärts", func() { fm.travel(1) })
	toolbar.PackStart(fm.back, false, false, 0)
	toolbar.PackStart(fm.forward, false, false, 0)
	toolbar.PackStart(fm.control("⌃", "Übergeordneter Ordner", func() { fm.navigate(filepath.Dir(fm.current), true) }), false, false, 0)
	toolbar.PackStart(fm.control("↻", "Aktualisieren", fm.render), false, false, 0)
	fm.path, _ = gtk.EntryNew()
	menuClass(fm.path, "fm-path")
	fm.path.SetHExpand(true)
	fm.path.Connect("activate", func() {
		value, _ := fm.path.GetText()
		value = strings.Replace(value, "~", fm.home, 1)
		if !filepath.IsAbs(value) {
			value = filepath.Join(fm.current, value)
		}
		fm.navigate(value, true)
	})
	toolbar.PackStart(fm.path, true, true, 0)

	root.PackStart(toolbar, false, false, 0)
	main, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	menuClass(main, "fm-main")
	sidebar, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	menuClass(sidebar, "fm-sidebar")
	sidebar.SetSizeRequest(170, -1)
	sidebar.PackStart(menuLabel("Angepinnt", "fm-section-label"), false, false, 5)
	fm.addPlace(sidebar, "Home", home, "user-home-symbolic")
	for _, place := range filePlaces {
		fm.addPlace(sidebar, place.name, filepath.Join(home, place.folder), place.icon)
	}
	sidebar.PackStart(menuLabel("Laufwerke", "fm-section-label"), false, false, 10)
	fm.addPlace(sidebar, "System", "/", "drive-harddisk")
	var disk syscall.Statfs_t
	if syscall.Statfs(home, &disk) == nil {
		free := float64(disk.Bavail) * float64(disk.Bsize) / (1024 * 1024 * 1024)
		sidebar.PackStart(menuLabel(fmt.Sprintf("%.1f GB verfügbar", free), "fm-muted"), false, false, 4)
	}
	sidebar.PackStart(menuLabel("Tags", "fm-section-label"), false, false, 10)
	for _, name := range []string{"Work", "Games", "Files", "Important", "Movies"} {
		tagPath := filepath.Join(home, "Documents", "Tags", name)
		_ = os.MkdirAll(tagPath, 0755)
		fm.addPlace(sidebar, name, tagPath, "folder")

	}
	toggle, _ := gtk.CheckButtonNewWithLabel("Versteckte Dateien")
	menuClass(toggle, "fm-hidden-toggle")
	toggle.Connect("toggled", func() { fm.hidden = toggle.GetActive(); fm.render() })
	sidebar.PackEnd(toggle, false, false, 6)
	sideScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	sideScroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	sideScroll.SetSizeRequest(180, 100)
	sideScroll.Add(sidebar)
	main.PackStart(sideScroll, false, false, 0)
	fm.content, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	menuClass(fm.content, "fm-content")
	fm.content.SetHExpand(true)
	fm.title = menuLabel("Home", "fm-page-title")
	fm.title.SetXAlign(0.5)
	fm.content.PackStart(fm.title, false, false, 8)
	fm.search, _ = gtk.EntryNew()
	win.Connect("key-press-event", func(_ *gtk.Window, event *gdk.Event) bool {
		if focus, err := win.GetFocus(); err == nil {
			if _, textEntry := focus.(*gtk.Entry); textEntry {
				return false
			}
		}
		if fm.path.HasFocus() || fm.search.HasFocus() {
			return false
		}
		key := gdk.EventKeyNewFromEvent(event)
		if key.State()&uint(gdk.CONTROL_MASK) == 0 {
			return false
		}
		switch key.KeyVal() {
		case 't', 'T':
			if fm.tabsBar != nil {
				fm.openTab(fm.home)
			}
		case 'w', 'W':
			if fm.tabsBar != nil {
				fm.closeTab(fm.activeTab)
			}
		case 'c', 'C':
			fm.copySelection(false)
		case 'x', 'X':
			fm.copySelection(true)
		case 'v', 'V':
			fm.pasteInto(fm.current)
		default:
			return false
		}
		return true
	})
	fm.search.SetPlaceholderText("Suchen …")
	menuClass(fm.search, "fm-search")
	fm.search.SetWidthChars(20)
	fm.search.SetHAlign(gtk.ALIGN_CENTER)
	searchVersion := uint64(0)
	fm.search.Connect("changed", func() {
		searchVersion++
		version := searchVersion
		glib.TimeoutAdd(140, func() bool {
			if fm.alive.Load() && version == searchVersion {
				fm.render()
			}
			return false
		})
	})
	toolbar.PackStart(fm.search, false, false, 0)
	main.PackStart(fm.content, true, true, 0)
	root.PackStart(main, true, true, 0)
	fm.status = menuLabel("", "fm-status")
	footer, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	menuClass(footer, "fm-footer")
	fm.status.SetHExpand(true)
	fm.status.SetMaxWidthChars(36)
	fm.status.SetLineWrap(true)
	fm.status.SetXAlign(0)
	footer.PackStart(fm.status, true, true, 0)
	footer.PackStart(fm.control("+", "Neuer Ordner", fm.newFolder), false, false, 0)
	footer.PackStart(fm.control("☆", "Ordner als Favorit merken", fm.toggleFavorite), false, false, 0)
	footer.PackStart(fm.control("⇅", "Sortierreihenfolge umkehren", func() { fm.reverseSort = !fm.reverseSort; fm.render() }), false, false, 0)
	footer.PackStart(fm.control("▦", "Kompakte Ansicht umschalten", func() { fm.compact = !fm.compact; fm.render() }), false, false, 0)
	footer.PackStart(fm.control("⧉", "Kopieren", func() { fm.copySelection(false) }), false, false, 0)
	footer.PackStart(fm.control("▤", "Einfügen", func() { fm.pasteInto(fm.current) }), false, false, 0)
	footer.PackStart(fm.control("✂", "Ausschneiden", func() { fm.copySelection(true) }), false, false, 0)
	root.PackEnd(footer, false, false, 0)
	if picker != nil {
		fm.addPickerControls(root)
	}
	fm.overlay, _ = gtk.OverlayNew()
	fm.overlay.Add(root)
	win.Add(resizableGlass(win, fm.overlay))
	registerWindowDrop(win, fm.receiveWindowDrop)
	fm.loadFavorites()
	if path == "" {
		path = home
	}
	if strings.HasPrefix(path, "file://") {
		if parsed, err := url.Parse(path); err == nil {
			path = parsed.Path
		}
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	if fm.tabsBar != nil {
		fm.tabs = []explorerTab{{path: path, historyAt: -1}}
	}
	fm.navigate(path, true)
	win.ShowAll()
	return fm
}

func (fm *glassFileManager) control(text, hint string, action func()) *gtk.Button {
	button, _ := gtk.ButtonNew()
	icons := map[string]string{"‹": "go-previous-symbolic", "›": "go-next-symbolic", "⌃": "go-up-symbolic", "↻": "view-refresh-symbolic", "+": "list-add-symbolic", "☆": "starred-symbolic", "⇅": "view-sort-ascending-symbolic", "▦": "view-grid-symbolic", "⧉": "edit-copy-symbolic", "▤": "edit-paste-symbolic", "✂": "edit-cut-symbolic"}
	if icon, ok := icons[text]; ok {
		button.Add(fmIcon(icon, gtk.ICON_SIZE_MENU))
	} else {
		button.Add(menuLabel(text, ""))
	}
	menuClass(button, "fm-control")
	button.SetTooltipText(hint)
	button.Connect("clicked", action)
	return button
}

func fmIcon(name string, size gtk.IconSize) *gtk.Image {
	image, _ := gtk.ImageNewFromIconName(name, size)
	if size == gtk.ICON_SIZE_DIALOG {
		image.SetPixelSize(46)
	}
	return image
}

func tagFolderIcon(index int) *gtk.Box {
	folder, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	folder.SetHAlign(gtk.ALIGN_CENTER)
	menuClass(folder, "fm-folder-icon")
	tab, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	tab.SetHAlign(gtk.ALIGN_START)
	tab.SetSizeRequest(21, 7)
	menuClass(tab, "fm-folder-tab")
	menuClass(tab, fmt.Sprintf("fm-folder-color-%d", index))
	front, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	front.SetSizeRequest(48, 32)
	menuClass(front, "fm-folder-front")
	menuClass(front, fmt.Sprintf("fm-folder-color-%d", index))
	folder.PackStart(tab, false, false, 0)
	folder.PackStart(front, false, false, 0)
	return folder
}

func (fm *glassFileManager) addPlace(sidebar *gtk.Box, name, path, icon string) {
	button, _ := gtk.ButtonNew()
	menuClass(button, "fm-place")
	row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if strings.HasPrefix(path, filepath.Join(fm.home, "Documents", "Tags")+string(os.PathSeparator)) {
		dot := menuLabel("●", "fm-tag-dot")
		fm.styleTagDot(dot, path)
		row.PackStart(dot, false, false, 0)
	} else {
		row.PackStart(fmIcon(icon, gtk.ICON_SIZE_MENU), false, false, 0)
	}
	row.PackStart(menuLabel(name, "fm-place-label"), false, false, 0)
	button.Add(row)
	button.Connect("clicked", func() { fm.search.SetText(""); fm.navigate(path, true) })
	fm.bindFileContext(button, path)
	fm.enableFileDrop(button, path)
	sidebar.PackStart(button, false, false, 0)
	fm.sidebar[path] = button
}

func (fm *glassFileManager) navigate(path string, remember bool) {
	path, err := filepath.Abs(path)
	if err != nil {
		fm.status.SetText(err.Error())
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		fm.status.SetText("Dieser Ordner kann nicht geöffnet werden.")
		return
	}
	if _, err := os.ReadDir(path); err != nil {
		fm.status.SetText("Für diesen Ordner fehlen Leserechte.")
		return
	}
	fm.current = path
	if remember {
		fm.history = append(fm.history[:fm.historyAt+1], path)
		fm.historyAt = len(fm.history) - 1
	}
	fm.back.SetSensitive(fm.historyAt > 0)
	fm.forward.SetSensitive(fm.historyAt+1 < len(fm.history))
	fm.path.SetText(path)
	for location, button := range fm.sidebar {
		style, _ := button.GetStyleContext()
		style.RemoveClass("fm-place-active")
		if location == path {
			style.AddClass("fm-place-active")
		}
	}
	fm.saveTab()
	fm.renderTabs()
	fm.render()
}
func (fm *glassFileManager) travel(delta int) {
	next := fm.historyAt + delta
	if next < 0 || next >= len(fm.history) {
		return
	}
	fm.historyAt = next
	fm.search.SetText("")
	fm.navigate(fm.history[next], false)
}

func (fm *glassFileManager) render() {
	if !fm.alive.Load() {
		return
	}
	atomic.AddUint64(&fm.generation, 1)
	fm.selection = nil
	fm.pickerSelection = nil
	fm.pickerSelection = nil
	if fm.current == "" {
		return
	}
	if fm.view != nil {
		fm.view.ToWidget().Destroy()
		fm.view = nil
	}
	query, _ := fm.search.GetText()
	if fm.picker == nil && fm.current == fm.home && query == "" {
		fm.title.SetText("Home")
		fm.renderHome()
		return
	}
	title := filepath.Base(fm.current)
	for _, place := range filePlaces {
		if fm.current == filepath.Join(fm.home, place.folder) {
			title = place.name
		}
	}
	if fm.current == "/" {
		title = "Debian"
	}
	fm.title.SetText(title)
	entries, err := os.ReadDir(fm.current)
	if err != nil {
		fm.status.SetText(err.Error())
		return
	}
	imageCount, fileCount := 0, 0
	for _, entry := range entries {
		if !entry.IsDir() && (fm.hidden || !strings.HasPrefix(entry.Name(), ".")) {
			if fm.picker != nil && !fm.picker.accepts(filepath.Join(fm.current, entry.Name())) {
				continue
			}
			fileCount++
			if isFileImage(entry.Name()) {
				imageCount++
			}
		}
	}
	fm.gallery = imageCount > 0 && imageCount == fileCount && !fm.compact
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		if fm.reverseSort {
			return strings.ToLower(entries[i].Name()) > strings.ToLower(entries[j].Name())
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	flow, _ := gtk.FlowBoxNew()
	menuClass(flow, "fm-file-grid")
	flow.SetSelectionMode(gtk.SELECTION_MULTIPLE)
	if fm.picker != nil {
		flow.SetSelectionMode(gtk.SELECTION_SINGLE)
	}
	if fm.picker != nil && fm.picker.Multiple {
		flow.SetSelectionMode(gtk.SELECTION_MULTIPLE)
	}
	flow.SetActivateOnSingleClick(false)
	flow.SetMaxChildrenPerLine(6)
	if fm.gallery {
		flow.SetMaxChildrenPerLine(4)
		menuClass(flow, "fm-gallery-grid")
	}
	flow.SetMinChildrenPerLine(1)
	flow.SetColumnSpacing(12)
	flow.SetRowSpacing(12)
	flow.SetVAlign(gtk.ALIGN_START)
	paths := []string{}
	for _, entry := range entries {
		if !fm.hidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(query)) {
			continue
		}
		path := filepath.Join(fm.current, entry.Name())
		if fm.picker != nil && !entry.IsDir() && (fm.picker.Directory || !fm.picker.accepts(path)) {
			continue
		}
		paths = append(paths, path)
		tileEvents, _ := gtk.EventBoxNew()
		tileEvents.SetVisibleWindow(false)
		tileEvents.Add(fm.fileTile(path, entry.Name(), entry.IsDir()))
		flow.Insert(tileEvents, -1)
		child := flow.GetChildAtIndex(len(paths) - 1)
		fm.enableFileDrag(tileEvents, path, flow, child)
		if entry.IsDir() {
			fm.enableFileDrop(tileEvents, path)
		}
		child.AddEvents(int(gdk.BUTTON_PRESS_MASK))
		tileEvents.AddEvents(int(gdk.BUTTON_PRESS_MASK))
		tileEvents.Connect("button-press-event", func(_ *gtk.EventBox, event *gdk.Event) bool {
			if gdk.EventButtonNewFromEvent(event).Button() != 3 {
				return false
			}
			if !child.IsSelected() {
				flow.UnselectAll()
				flow.SelectChild(child)
			}
			fm.contextMenu(path, event)
			return true
		})
	}
	flow.Connect("child-activated", func(_ *gtk.FlowBox, child *gtk.FlowBoxChild) {
		index := child.GetIndex()
		if index >= 0 && index < len(paths) {
			fm.open(paths[index])
		}
	})
	flow.Connect("selected-children-changed", func() {
		fm.selection = nil
		for _, child := range flow.GetSelectedChildren() {
			i := child.GetIndex()
			if i >= 0 && i < len(paths) {
				fm.selection = append(fm.selection, paths[i])
			}
		}
		if fm.picker != nil {
			fm.pickerSelection = append([]string(nil), fm.selection...)
			if fm.picker.Save && len(fm.pickerSelection) == 1 {
				if info, err := os.Stat(fm.pickerSelection[0]); err == nil && !info.IsDir() {
					fm.pickerName.SetText(filepath.Base(fm.pickerSelection[0]))
				}
			}
		}
	})
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scroll.Add(flow)
	fm.enableFileDrop(scroll, fm.current)
	scroll.AddEvents(int(gdk.BUTTON_PRESS_MASK))
	scroll.Connect("button-press-event", func(_ *gtk.ScrolledWindow, event *gdk.Event) bool {
		if gdk.EventButtonNewFromEvent(event).Button() != 3 {
			return false
		}
		fm.contextMenu("", event)
		return true
	})
	fm.view = scroll
	fm.content.PackStart(scroll, true, true, 0)
	scroll.ShowAll()
	fm.status.SetText(fmt.Sprintf("%d Elemente  ·  Doppelklick zum Öffnen", len(paths)))
	if len(paths) == 0 {
		fm.status.SetText("Keine Dateien gefunden. Du kannst hier einen neuen Ordner anlegen.")
	}
}

func (fm *glassFileManager) fileTile(path, name string, folder bool) *gtk.Box {
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	menuClass(box, "fm-file-tile")
	box.SetSizeRequest(120, 112)
	if fm.gallery && !folder {
		box.SetSizeRequest(148, 128)
		menuClass(box, "fm-gallery-tile")
	}
	box.SetTooltipText(path)
	var image *gtk.Image
	if folder {
		image = fmIcon("folder", gtk.ICON_SIZE_DIALOG)
	} else if isFileImage(path) {
		width, height := 128, 82
		if fm.gallery {
			width = 148
			height = 100
		}
		image = fmIcon("image-x-generic", gtk.ICON_SIZE_DIALOG)
		generation := atomic.LoadUint64(&fm.generation)
		target := image
		go func() {
			thumbnailSlots <- struct{}{}
			defer func() { <-thumbnailSlots }()
			if atomic.LoadUint64(&fm.generation) != generation {
				return
			}
			pixbuf, err := gdk.PixbufNewFromFileAtScale(path, width, height, true)
			if err != nil {
				return
			}
			glib.IdleAdd(func() {
				if atomic.LoadUint64(&fm.generation) == generation {
					target.SetFromPixbuf(pixbuf)
				}
			})
		}()
	}
	if image == nil {
		icon := "text-x-generic"
		switch strings.ToLower(filepath.Ext(path)) {
		case ".deb":
			icon = "package-x-generic"
		case ".mp3", ".flac", ".wav":
			icon = "audio-x-generic"
		case ".mp4", ".mkv", ".webm":
			icon = "video-x-generic"
		case ".pdf":
			icon = "application-pdf"
		}
		image = fmIcon(icon, gtk.ICON_SIZE_DIALOG)
	}
	image.SetHAlign(gtk.ALIGN_CENTER)
	box.PackStart(image, true, true, 0)
	label := menuLabel(name, "fm-file-name")
	label.SetXAlign(0.5)
	label.SetLineWrap(true)
	label.SetMaxWidthChars(18)
	box.PackStart(label, false, false, 0)
	return box
}

func isFileImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

func (fm *glassFileManager) open(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fm.status.SetText("Die Datei ist nicht mehr verfügbar.")
		return
	}
	if info.IsDir() {
		fm.search.SetText("")
		fm.navigate(path, true)
		return
	}
	if fm.picker != nil {
		if fm.picker.Save {
			fm.pickerName.SetText(filepath.Base(path))
			return
		}
		if !fm.picker.Multiple {
			fm.pickerSelection = []string{path}
			fm.submitPicker()
		}
		return
	}
	if isFileImage(path) {
		fm.previewImage(path)
		return
	}
	if strings.EqualFold(filepath.Ext(path),".exe") || strings.EqualFold(filepath.Ext(path),".msi") {
		binary,err:=os.Executable();if err==nil{err=exec.Command(binary,"--windows",path).Start()};if err!=nil{fm.status.SetText("Die Windows-App konnte nicht geöffnet werden.")};return
	}
	if strings.EqualFold(filepath.Ext(path), ".deb") {
		binary, err := os.Executable()
		if err == nil {
			err = exec.Command(binary, "--install", path).Start()
		}
		if err != nil {
			fm.status.SetText("Der Paketinstaller konnte nicht geöffnet werden.")
		}
		return
	}
	if err := exec.Command("xdg-open", path).Start(); err != nil {
		fm.status.SetText("Die Datei konnte nicht geöffnet werden.")
	}
}

func (fm *glassFileManager) previewImage(path string) {
	preview, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	title := "Bild — Galerie"
	preview.SetTitle(title)
	preview.SetDefaultSize(880, 590)
	root, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(root, "fm-shell")
	menuClass(root, "fm-explorer")
	root.PackStart(glassWindowHeader(preview, title), false, false, 0)
	paths := []string{}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, entry := range entries {
		if !entry.IsDir() && isFileImage(entry.Name()) && (fm.hidden || !strings.HasPrefix(entry.Name(), ".")) {
			paths = append(paths, filepath.Join(filepath.Dir(path), entry.Name()))
		}
	}
	index := 0
	for i, candidate := range paths {
		if candidate == path {
			index = i
			break
		}
	}
	if len(paths) == 0 {
		paths = []string{path}
	}
	image, _ := gtk.ImageNew()
	root.PackStart(image, true, true, 10)
	bar, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 14)
	menuClass(bar, "picker-controls")
	previous, _ := gtk.ButtonNewWithLabel("‹")
	next, _ := gtk.ButtonNewWithLabel("›")
	menuClass(previous, "fm-control")
	menuClass(next, "fm-control")
	previous.SetTooltipText("Vorheriges Bild")
	next.SetTooltipText("Nächstes Bild")
	caption := menuLabel("", "fm-card-title")
	caption.SetHExpand(true)
	caption.SetXAlign(.5)
	var previewGeneration uint64
	var previewAlive atomic.Bool
	previewAlive.Store(true)
	preview.Connect("destroy", func() { previewAlive.Store(false); atomic.AddUint64(&previewGeneration, 1) })
	update := func() {
		generation := atomic.AddUint64(&previewGeneration, 1)
		current := paths[index]
		image.SetFromIconName("image-loading-symbolic", gtk.ICON_SIZE_DIALOG)
		go func() {
			thumbnailSlots <- struct{}{}
			defer func() { <-thumbnailSlots }()
			pixbuf, err := gdk.PixbufNewFromFileAtScale(current, 830, 450, true)
			if err != nil {
				return
			}
			glib.IdleAdd(func() {
				if previewAlive.Load() && atomic.LoadUint64(&previewGeneration) == generation {
					image.SetFromPixbuf(pixbuf)
				}
			})
		}()
		caption.SetText(fmt.Sprintf("%d von %d · %s", index+1, len(paths), filepath.Base(paths[index])))
		previous.SetSensitive(index > 0)
		next.SetSensitive(index < len(paths)-1)
	}
	previous.Connect("clicked", func() {
		if index > 0 {
			index--
			update()
		}
	})
	next.Connect("clicked", func() {
		if index < len(paths)-1 {
			index++
			update()
		}
	})
	bar.PackStart(previous, false, false, 0)
	bar.PackStart(caption, true, true, 0)
	bar.PackEnd(next, false, false, 0)
	root.PackEnd(bar, false, false, 0)
	preview.Add(root)
	update()
	preview.ShowAll()
}
func (fm *glassFileManager) renderHome() {
	columns, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	left, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	pinned, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	menuClass(pinned, "fm-home-card")
	pinned.PackStart(menuLabel("✦  Angepinnt", "fm-card-title"), false, false, 0)
	grid, _ := gtk.FlowBoxNew()
	grid.SetSelectionMode(gtk.SELECTION_NONE)
	grid.SetMinChildrenPerLine(1)
	grid.SetMaxChildrenPerLine(5)
	grid.SetColumnSpacing(3)
	for _, place := range filePlaces {
		place := place
		button, _ := gtk.ButtonNew()
		menuClass(button, "fm-home-folder")
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
		box.PackStart(fmIcon(place.icon, gtk.ICON_SIZE_DIALOG), false, false, 0)
		label := menuLabel(place.name, "fm-small-name")
		label.SetXAlign(0.5)
		box.PackStart(label, false, false, 0)
		button.Add(box)
		button.Connect("clicked", func() { fm.navigate(filepath.Join(fm.home, place.folder), true) })
		fm.bindFileContext(button, filepath.Join(fm.home, place.folder))
		fm.enableFileDrop(button, filepath.Join(fm.home, place.folder))
		grid.Insert(button, -1)
	}
	pinned.PackStart(grid, false, false, 0)
	left.PackStart(pinned, false, false, 0)
	tags, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	menuClass(tags, "fm-home-card")
	tags.PackStart(menuLabel("●  Tag-Ordner", "fm-card-title"), false, false, 0)
	row, _ := gtk.FlowBoxNew()
	row.SetSelectionMode(gtk.SELECTION_NONE)
	row.SetMinChildrenPerLine(1)
	row.SetMaxChildrenPerLine(5)
	for _, name := range []string{"Work", "Games", "Files", "Important", "Movies"} {
		name := name
		button, _ := gtk.ButtonNew()
		menuClass(button, "fm-home-folder")
		box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
		folder := menuLabel("●", "fm-tag-dot")
		fm.styleTagDot(folder, filepath.Join(fm.home, "Documents", "Tags", name))
		box.PackStart(folder, false, false, 0)
		label := menuLabel(name, "fm-small-name")
		label.SetXAlign(0.5)
		box.PackStart(label, false, false, 0)
		button.Add(box)
		tagPath := filepath.Join(fm.home, "Documents", "Tags", name)
		_ = os.MkdirAll(tagPath, 0755)
		fm.bindFileContext(button, tagPath)
		fm.enableFileDrop(button, tagPath)
		button.Connect("clicked", func() {
			path := filepath.Join(fm.home, "Documents", "Tags", name)
			if err := os.MkdirAll(path, 0755); err == nil {
				fm.navigate(path, true)
			}
		})
		row.Insert(button, -1)
	}
	tags.PackStart(row, false, false, 0)
	left.PackStart(tags, false, false, 0)
	columns.PackStart(left, true, true, 0)
	recent, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 14)
	menuClass(recent, "fm-home-card")
	recent.SetSizeRequest(166, -1)
	recent.PackStart(menuLabel("Zuletzt geändert", "fm-card-title"), false, false, 0)
	type item struct {
		path     string
		modified time.Time
	}
	items := []item{}
	for _, place := range filePlaces {
		dir := filepath.Join(fm.home, place.folder)
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if info, err := entry.Info(); err == nil {
				items = append(items, item{filepath.Join(dir, entry.Name()), info.ModTime()})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].modified.After(items[j].modified) })
	for index, item := range items {
		if index >= 4 {
			break
		}
		recent.PackStart(fm.homeItem(item.path), false, false, 0)
	}
	if len(items) == 0 {
		recent.PackStart(menuLabel("Hier erscheinen deine\nzuletzt geänderten Dateien.", "fm-muted"), false, false, 0)
	}
	columns.PackStart(recent, false, false, 0)
	favorites, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 14)
	menuClass(favorites, "fm-home-card")
	favorites.SetSizeRequest(166, -1)
	favorites.PackStart(menuLabel("Favoriten", "fm-card-title"), false, false, 0)
	for index, path := range fm.favorites {
		if index < 4 {
			favorites.PackStart(fm.homeItem(path), false, false, 0)
		}
	}
	favorites.PackEnd(menuLabel("Mit ☆ Ordner merken", "fm-muted"), false, false, 0)
	columns.PackStart(favorites, false, false, 0)
	scroll, _ := gtk.ScrolledWindowNew(nil, nil)
	scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scroll.Add(columns)
	scroll.Connect("size-allocate", func() {
		if scroll.GetAllocatedWidth() < 720 {
			columns.SetOrientation(gtk.ORIENTATION_VERTICAL)
		} else {
			columns.SetOrientation(gtk.ORIENTATION_HORIZONTAL)
		}
	})
	fm.view = scroll
	fm.content.PackStart(scroll, true, true, 12)
	scroll.ShowAll()
	fm.status.SetText("Deine Dateien. Dein Platz.  ·  Ordner anklicken zum Öffnen")
}

func (fm *glassFileManager) homeItem(path string) *gtk.Button {
	button, _ := gtk.ButtonNew()
	menuClass(button, "fm-home-item")
	button.SetTooltipText(path)
	row, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	icon := "text-x-generic"
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		icon = "folder"
	}
	if isFileImage(path) {
		icon = "image-x-generic"
	}
	if strings.HasPrefix(path, filepath.Join(fm.home, "Documents", "Tags")+string(os.PathSeparator)) {
		dot := menuLabel("●", "fm-tag-dot")
		fm.styleTagDot(dot, path)
		row.PackStart(dot, false, false, 0)
	} else {
		row.PackStart(fmIcon(icon, gtk.ICON_SIZE_MENU), false, false, 0)
	}
	text, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 3)
	name := menuLabel(filepath.Base(path), "fm-recent-name")
	name.SetMaxWidthChars(17)
	name.SetLineWrap(true)
	text.PackStart(name, false, false, 0)
	text.PackStart(menuLabel(filepath.Base(filepath.Dir(path)), "fm-muted"), false, false, 0)
	row.PackStart(text, true, true, 0)
	button.Add(row)
	button.Connect("clicked", func() { fm.open(path) })
	fm.enableFileDrag(button, path, nil, nil)
	fm.bindFileContext(button, path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		fm.enableFileDrop(button, path)
	}
	return button
}

func (fm *glassFileManager) favoritesFile() string {
	return filepath.Join(fm.home, ".local", "state", "miracleos-files", "favorites.json")
}
func (fm *glassFileManager) loadFavorites() {
	data, err := os.ReadFile(fm.favoritesFile())
	if err == nil && json.Unmarshal(data, &fm.favorites) == nil {
		return
	}
	fm.favorites = []string{filepath.Join(fm.home, "Downloads"), filepath.Join(fm.home, "Pictures")}
}
func (fm *glassFileManager) toggleFavorite() {
	found := false
	for index, path := range fm.favorites {
		if path == fm.current {
			fm.favorites = append(fm.favorites[:index], fm.favorites[index+1:]...)
			found = true
			break
		}
	}
	if !found {
		fm.favorites = append(fm.favorites, fm.current)
	}
	path := fm.favoritesFile()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fm.status.SetText("Favoriten konnten nicht gespeichert werden.")
		return
	}
	data, _ := json.Marshal(fm.favorites)
	if err := os.WriteFile(path+".tmp", data, 0600); err == nil {
		os.Rename(path+".tmp", path)
	}
	fm.status.SetText("Favoriten aktualisiert.")
}

func (fm *glassFileManager) newFolder() {
	dialog, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	dialog.SetTitle("Neuer Ordner")
	dialog.SetTransientFor(fm.window)
	dialog.SetModal(true)
	dialog.SetDefaultSize(340, 180)
	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	menuClass(box, "fm-dialog")
	box.PackStart(menuLabel("Neuer Ordner", "fm-card-title"), false, false, 0)
	entry, _ := gtk.EntryNew()
	entry.SetPlaceholderText("Ordnername")
	menuClass(entry, "fm-path")
	box.PackStart(entry, false, false, 0)
	message := menuLabel("", "fm-muted")
	box.PackStart(message, false, false, 0)
	create, _ := gtk.ButtonNewWithLabel("Anlegen")
	menuClass(create, "dialog-action")
	menuClass(create, "dialog-primary")
	create.Connect("clicked", func() {
		name, _ := entry.GetText()
		name = strings.TrimSpace(name)
		if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
			message.SetText("Bitte einen gültigen Ordnernamen eingeben.")
			return
		}
		if err := os.Mkdir(filepath.Join(fm.current, name), 0755); err != nil {
			message.SetText("Der Ordner existiert bereits oder kann nicht angelegt werden.")
			return
		}
		dialog.Destroy()
		fm.render()
	})
	box.PackEnd(create, false, false, 0)
	shell, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	menuClass(shell, "fm-shell")
	menuClass(shell, "glass-dialog")
	shell.PackStart(glassDialogHeader(dialog, "Neuer Ordner"), false, false, 0)
	shell.PackStart(box, true, true, 0)
	cancel, _ := gtk.ButtonNewWithLabel("Abbrechen")
	menuClass(cancel, "dialog-action")
	cancel.Connect("clicked", func() { dialog.Destroy() })
	box.PackEnd(cancel, false, false, 0)
	entry.Connect("activate", func() { create.Clicked() })
	dialog.Connect("key-press-event", func(_ interface{}, event *gdk.Event) bool {
		if gdk.EventKeyNewFromEvent(event).KeyVal() == gdk.KEY_Escape {
			dialog.Destroy()
			return true
		}
		return false
	})
	dialog.Add(shell)
	dialog.ShowAll()
}
