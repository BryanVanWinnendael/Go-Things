package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/emersion/go-imap/v2"
)

// appDataDir is the folder in the user's config directory that holds all of
// the app's state: config, cache, logos and the crash log.
const appDataDir = "Go Mail"

// App is the whole application: connection state, mail data and UI state.
// mu guards everything a background goroutine touches; netMu serialises the
// IMAP connection, which takes one command at a time.
type App struct {
	mu    sync.Mutex
	netMu sync.Mutex
	ctx   atomic.Pointer[ui.Context]

	sess          *Session
	acct          Account
	folders       []Folder
	folder        string
	msgs          []Message
	sel           int
	body          string
	bodyHTML      string
	bodyUID       imap.UID
	viewAsText    bool
	viewingMsg    *Message
	viewingFolder string
	viewingUID    imap.UID
	pendingScroll bool
	scrollState   ui.ScrollState
	loadingMore   bool
	loadedLimit   int
	hasMore       bool
	cache         map[string]*folderCache

	connected   bool
	connecting  bool
	switching   bool
	switchFrom  int
	connID      int
	loadingMsgs bool
	bodyLoading bool
	errMsg      string
	note        string

	accounts         []Account
	active           int
	accountOpen      bool
	ctxMenu          contextMenuState
	ctxMenuAnim      float32
	dialogAnim       float32
	confirmAnim      float32
	searchOpen       bool
	searchQuery      string
	searchThemeMode  bool
	themePreviewBase string
	themePreviewLast string
	filterQuery      string
	bgFetching       bool
	fetchGen         int
	sharedMailbox    bool
	diskCache        diskCache
	logos            logoCache
	appIcon          *ui.Bitmap
	keepAliveGen     int
	keepAliveStop    chan struct{}
	reconnecting     bool

	filterPanelOpen    bool
	filterFlagged      bool
	filterUnread       bool
	filterFrom         string
	filterTo           string
	filterDateFrom     string
	filterDateTo       string
	filterDateFromOpen bool
	filterDateToOpen   bool
	dateCursor         time.Time
	dateCursorKey      string

	dark               bool
	themeSettings      ThemeSettings
	split              float32
	listState          ui.ListState
	folderListState    ui.ListState
	folderScrollState  ui.ScrollState
	sidebarW           float32
	sidebarCollapsed   bool
	sidebarAnimW       float32
	sidebarSel         string
	showCompose        bool
	composeTo          string
	composeCc          string
	composeSub         string
	composeBody        string
	composeQuote       string
	composeDoc         *richtext.Document
	composeColor       ui.Color
	composeAttachments []composeAttachment
	composeErr         string
	sending            bool

	form     Account
	setupErr string
	busy     bool

	// Theme state, ported from Go Post.
	palette      palette
	systemDark   bool
	customThemes []CustomTheme

	mainWindow  *mygo.Window
	readerView  *mygo.WebView
	settingsWin *mygo.Window
	settingsTab string
	settingsW   float32
	sigEd       *signatureEditor

	pendingSettings    bool
	pendingThemePicker bool
	pendingMaterials   atomic.Bool

	themePickerOpen         bool
	themePickerAnchor       string
	themePickerDraft        CustomTheme
	themePickerOriginal     CustomTheme
	themePickerIndex        int
	themePickerPoint        int
	themePickerWin          *mygo.Window
	themePickerHex          string
	themePickerSurface      string
	themePickerSurfaceError string
	themePickerTarget       string
	themePickerAccentHex    string

	materialOpen bool

	confirmTitle string
	confirmInfo  string
	confirmFn    func()

	confirmClearCache bool
}

// folderCache keeps a folder's loaded pages so switching away and back does
// not throw them away or refetch from the server.
type folderCache struct {
	msgs        []Message
	loadedLimit int
	hasMore     bool
	sel         int
	scrollY     float32
}

type provider struct {
	name, imapHost, imapPort, smtpHost, smtpPort string
}

var providers = []provider{
	{"Gmail", "imap.gmail.com", "993", "smtp.gmail.com", "587"},
	{"Outlook", "outlook.office365.com", "993", "smtp.office365.com", "587"},
	{"iCloud", "imap.mail.me.com", "993", "smtp.mail.me.com", "587"},
	{"Yahoo", "imap.mail.yahoo.com", "993", "smtp.mail.yahoo.com", "465"},
	{"Fastmail", "imap.fastmail.com", "993", "smtp.fastmail.com", "465"},
}

var (
	iconMail        = mustIcon(`<rect x="2.5" y="4.5" width="19" height="15" rx="2.5"/><path d="m3 7 8.2 5.5a1.5 1.5 0 0 0 1.6 0L21 7"/>`)
	iconInbox       = mustIcon(`<path d="M22 12h-6l-2 3h-4l-2-3H2"/><path d="M5.5 5.1 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.5-6.9A2 2 0 0 0 16.8 4H7.2a2 2 0 0 0-1.7 1.1Z"/>`)
	iconSent        = mustIcon(`<path d="M22 2 11 13"/><path d="M22 2 15 22l-4-9-9-4Z"/>`)
	iconDrafts      = mustIcon(`<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/>`)
	iconTrash       = mustIcon(`<path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>`)
	iconJunk        = mustIcon(`<path d="M12 2 2 22h20L12 2Z"/><path d="M12 9v5"/><path d="M12 18h.01"/>`)
	iconArchive     = mustIcon(`<rect x="2" y="3" width="20" height="5" rx="1.5"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/>`)
	iconFolder      = mustIcon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`)
	iconRefresh     = mustIcon(`<path d="M21 12a9 9 0 1 1-2.6-6.4L21 8"/><path d="M21 3v5h-5"/>`)
	iconPen         = mustIcon(`<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/>`)
	iconImage       = mustIcon(`<rect x="3" y="3" width="18" height="18" rx="2.5"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="m21 15-5-5L5 21"/>`)
	iconReply       = mustIcon(`<path d="M9 17 4 12l5-5"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/>`)
	iconEye         = mustIcon(`<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>`)
	iconMoon        = mustIcon(`<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>`)
	iconSun         = mustIcon(`<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>`)
	iconLogout      = mustIcon(`<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5"/><path d="M21 12H9"/>`)
	iconX           = mustIcon(`<path d="M18 6 6 18M6 6l12 12"/>`)
	iconFlag        = mustIcon(`<path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><line x1="4" y1="22" x2="4" y2="15"/>`)
	iconSearch      = mustIcon(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)
	iconFilter      = mustIcon(`<path d="M3 4h18l-7 8v6l-4 2v-8L3 4Z"/>`)
	iconCalendar    = mustIcon(`<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>`)
	iconSidebar     = mustIcon(`<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 3v18"/>`)
	iconMinimize    = mustIcon(`<path d="M5 12h14"/>`)
	iconMaximize    = mustIcon(`<rect x="5" y="5" width="14" height="14" rx="1.5"/>`)
	iconRestore     = mustIcon(`<rect x="7" y="7" width="12" height="12" rx="1.5"/><path d="M5 15V7a2 2 0 0 1 2-2h8"/>`)
	iconChevron     = mustIcon(`<path d="m9 6 6 6-6 6"/>`)
	iconChevronDown = mustIcon(`<path d="m6 9 6 6 6-6"/>`)
	iconLayers      = mustIcon(`<path d="m12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83Z"/><path d="m22 17.65-9.17 4.16a2 2 0 0 1-1.66 0L2 17.65"/><path d="m22 12.65-9.17 4.16a2 2 0 0 1-1.66 0L2 12.65"/>`)
	iconSparkles    = mustIcon(`<path d="M9.94 2.5 11 5.6l3.1 1.06L11 7.72 9.94 10.8 8.88 7.72 5.78 6.66 8.88 5.6Z"/><path d="M18 13l.8 2.2L21 16l-2.2.8L18 19l-.8-2.2L15 16l2.2-.8Z"/><path d="M5 15l.6 1.6L7.2 17l-1.6.6L5 19.2l-.6-1.6L2.8 17l1.6-.4Z"/>`)
	iconDot         = mustIcon(`<circle cx="12" cy="12" r="3.5"/>`)
	iconPlus        = mustIcon(`<path d="M12 5v14M5 12h14"/>`)
	iconMinus       = mustIcon(`<path d="M5 12h14"/>`)
	iconCheck       = mustIcon(`<path d="m5 12 5 5 9-11"/>`)
	iconCopy        = mustIcon(`<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>`)
	iconPalette     = mustIcon(`<circle cx="13.5" cy="6.5" r=".5"/><circle cx="17.5" cy="10.5" r=".5"/><circle cx="8.5" cy="7.5" r=".5"/><circle cx="6.5" cy="12.5" r=".5"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.9 0 1.5-.7 1.5-1.5 0-.4-.1-.8-.4-1.1-.3-.3-.4-.7-.4-1.1 0-.8.7-1.5 1.5-1.5H16c3.3 0 6-2.7 6-6 0-4.9-4.5-8.8-10-8.8Z"/>`)
	iconGear        = mustIcon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>`)
	iconInfo        = mustIcon(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>`)
)

func mustIcon(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`))
}

func folderIcon(kind string) *ui.SVG {
	switch kind {
	case "inbox":
		return iconInbox
	case "sent":
		return iconSent
	case "drafts":
		return iconDrafts
	case "trash":
		return iconTrash
	case "junk":
		return iconJunk
	case "flagged":
		return iconFlag
	case "archive":
		return iconArchive
	}
	return iconFolder
}

func (a *App) applyPalette(c *ui.Context) *ui.Theme {
	if a.themeSettings.Theme == "" || a.themeSettings.Theme == "system" {
		a.systemDark = c.Theme().Dark
	}
	p := resolvePalette(a.themeSettings, a.systemDark, a.customThemes)
	a.palette = p
	activePalette = p
	a.dark = p.Dark
	return applyTheme(c, p, a.themeSettings)
}

// paintChrome fills the window behind the solid stage. A chrome gradient lives
// here, not in the stage, so cards and the reading pane stay opaque.
func (a *App) paintChrome(c *ui.Context) {
	p := a.palette
	vibrancy := c.Vibrancy()
	if p.HasGradient {
		if vibrancy {
			c.Root().Background(p.Bg.Alpha(chromeScrimAlpha))
			c.Root().Draw(func(pt *ui.Painter, r ui.Rect) {
				paintChromeGradient(pt, r, p, chromeAcrylicGradientScale)
			})
		} else {
			c.Root().Background(p.Bg)
			c.Root().Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(r, p.Bg, 0)
				paintChromeGradient(pt, r, p, 2)
			})
		}
		return
	}
	c.Root().Draw(nil)
	if vibrancy {
		c.Root().Background(p.Bg.Alpha(chromeScrimAlpha))
	} else {
		c.Root().Background(p.Bg)
	}
}

// view is the window's frame.
func (a *App) view(c *ui.Context) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("view", r)
		}
	}()
	a.ctx.Store(c)
	a.flushPending()
	t := a.applyPalette(c)
	a.mu.Lock()
	// Infinite scroll: load more when near the bottom.
	if a.connected && a.hasMore && !a.loadingMore && !a.bgFetching && a.filterQuery == "" && a.scrollState.MaxY > 0 && a.scrollState.Y >= a.scrollState.MaxY-200 {
		a.loadingMore = true
		offset := a.loadedLimit
		a.loadedLimit += 100
		go func() {
			a.loadFolder(a.folder, true, offset)
			a.mu.Lock()
			a.loadingMore = false
			a.mu.Unlock()
		}()
	}
	defer a.mu.Unlock()
	a.paintChrome(c)
	c.OnShortcut(ui.Cmd, ui.KeySpace, func() {
		a.searchOpen = true
		a.searchQuery = ""
		a.invalidate()
	})
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		a.sidebarCollapsed = !a.sidebarCollapsed
		a.invalidate()
	})
	c.OnShortcut(ui.Cmd, ui.KeyT, func() {
		a.openSettings()
	})

	bar := c.TitleBar()
	height := bar.Height
	if height < 40 {
		height = 40
	}
	// Drive the palette's open animation from the window root every frame so it
	// plays even though MyGo lays overlays out after its transition pass. MyGo
	// returns the target immediately on a channel's first frame, so the channel
	// has to exist at 0 before the palette opens.
	paletteTarget := float32(0)
	if a.searchOpen {
		paletteTarget = 1
	}
	a.dialogAnim = c.Root().Animate("dialog", paletteTarget, 140*time.Millisecond)
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(height).Shrink(0).Padding(0, bar.Right+12, 0, bar.Left+16).Gap(9).
			AlignItems(ui.Center).DragWindow().Children(func() {
			widthTarget := float32(250)
			if a.sidebarCollapsed {
				widthTarget = 0
			}
			toggle := ui.ButtonBase(c.Key("toggleSidebar")).Size(30, 30).Radius(7).Center().Label("Toggle sidebar").Tooltip("Toggle sidebar").FocusRing(false)
			if toggle.Hovered() {
				toggle.Background(t.SurfaceHover)
			}
			a.sidebarAnimW = toggle.Animate("sidebar-width", widthTarget, 200*time.Millisecond)
			toggle.Children(func() {
				ui.Icon(c, iconSidebar).Size(14, 14).TextColor(t.TextMuted)
			})
			if toggle.Clicked() {
				a.sidebarCollapsed = !a.sidebarCollapsed
			}

			ui.Spacer(c).Grow(1)
			if a.connecting {
				ui.Text(c, "Connecting…").FontSize(11).TextColor(t.TextMuted)
			}
			a.windowControls(c, a.mainWindow)
		})
		// The whole app face swaps under a short fade, so connecting or
		// setting up does not teleport the window.
		state := "setup"
		if a.connected {
			state = "mail"
		} else if a.connecting {
			state = "loading"
		}
		ui.Column(c).Key("app-face:" + state).Grow(1).MinHeight(0).Clip().
			Transition(ui.ElementTransition{
				Duration: 170 * time.Millisecond,
				Enter:    &ui.Motion{Y: 5, Opacity: 0},
			}).Children(func() {
			switch {
			case a.connected:
				a.mailView(c)
			case a.connecting:
				a.loadingView(c)
			default:
				a.setupView(c)
			}
		})
		if a.searchOpen {
			a.searchPalette(c)
		}
	})
	a.contextMenu(c)
}

func (a *App) invalidate() {
	if c := a.ctx.Load(); c != nil {
		c.Invalidate()
	}
}

func (a *App) invalidateAll() {
	if c := a.ctx.Load(); c != nil {
		c.Invalidate()
	}
	a.refreshWindows()
}

// ---------------------------------------------------------------- setup ---

func (a *App) appIconBitmap() *ui.Bitmap {
	if a.appIcon != nil {
		return a.appIcon
	}
	if bmp, err := ui.DecodeBitmap(appIconPNG); err == nil {
		a.appIcon = bmp
	}
	return a.appIcon
}

func (a *App) loadingView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Center().Gap(14).Children(func() {
		if bmp := a.appIconBitmap(); bmp != nil {
			ui.Image(c, bmp).Size(54, 54).Radius(15)
		} else {
			ui.Box(c).Size(54, 54).Radius(15).Background(t.Accent).Center().Children(func() {
				ui.Icon(c, iconMail).FontSize(25).TextColor(ui.Hex("#ffffff"))
			})
		}
		ui.Text(c, "Connecting…").FontSize(15).FontWeight(600)
		if a.form.Email != "" {
			ui.Text(c, a.form.Email).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *App) setupView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(24).Center().Children(func() {
		ui.Column(c).Width(900).MaxWidthPercent(96).Padding(26).Gap(18).Background(paneColor(t)).Border(1, t.Border).Radius(16).
			Shadow(0, 20, 50, -24, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
			ui.Row(c).Gap(13).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(44, 44).Radius(12).Background(t.Accent).Center().Children(func() {
					ui.Icon(c, iconMail).FontSize(21).TextColor(ui.Hex("#ffffff"))
				})
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "Go Mail").FontSize(20).FontWeight(700)
					ui.Text(c, "Connect your mailbox to get started").FontSize(12.5).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				if iconButton(c, "setupSettings", iconGear).Clicked() {
					a.openSettings()
				}
				if iconButton(c, "theme", themeIcon(a.dark)).Clicked() {
					a.toggleLightDark()
				}
			})
			ui.Divider(c)
			ui.Row(c).Gap(32).AlignItems(ui.Start).Children(func() {
				a.accountForm(c)
				a.serverForm(c)
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Text(c, "QUICK SETUP").FontSize(10).FontWeight(600).LetterSpacing(1.2).TextColor(t.TextMuted)
				for _, p := range providers {
					p := p
					chip := ui.Button(c.Key("preset:"+p.name), p.name).Padding(5, 12).Radius(999).FontSize(12).
						Background(t.Surface).Border(1, t.Border)
					if chip.Clicked() {
						a.applyPreset(p)
					}
				}
			})
			if a.setupErr != "" {
				ui.Box(c).FillWidth().Padding(10, 14).Radius(9).Background(t.Danger.Alpha(0.1)).Children(func() {
					ui.Text(c, a.setupErr).FontSize(12).TextColor(t.Danger).Wrap()
				})
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if a.busy {
					ui.Text(c, "Connecting to your mail server…").FontSize(12).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "Settings are saved locally. Your password is not.").FontSize(11).TextColor(t.TextMuted)
				}
				ui.Spacer(c)
				if ui.Button(c.Key("quit"), "Quit").Padding(7, 14).Radius(8).Clicked() {
					mygo.App.Quit()
				}
				connect := ui.PrimaryButton(c.Key("connect"), "Connect").Padding(8, 22).Radius(8)
				if a.busy {
					connect.Disabled(true)
				}
				if connect.Clicked() && !a.busy {
					a.connect()
				}
			})
		})
	})
}

func (a *App) accountForm(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Gap(12).Children(func() {
		sectionLabel(c, "Your account")
		formField(c, "Name", func() {
			ui.TextInput(c, &a.form.Name).Placeholder("Alex Doe")
		})
		formField(c, "Email address", func() {
			ui.TextInput(c, &a.form.Email).Placeholder("you@example.com")
		})
		formField(c, "Password", func() {
			ui.TextInput(c, &a.form.Password).Password().Placeholder("App password")
		})
		ui.Text(c, "Gmail, iCloud and Yahoo need an app password from your account's security settings.").
			FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *App) serverForm(c *ui.Context) {
	ui.Column(c).Grow(1).Gap(12).Children(func() {
		sectionLabel(c, "Incoming mail (IMAP)")
		ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
			formField(c, "Server", func() {
				ui.TextInput(c, &a.form.IMAPHost).Placeholder("imap.example.com")
			}).Grow(1)
			formField(c, "Port", func() {
				ui.TextInput(c, &a.form.IMAPPort).Placeholder("993")
			}).Width(84)
		})
		sectionLabel(c, "Outgoing mail (SMTP)")
		ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
			formField(c, "Server", func() {
				ui.TextInput(c, &a.form.SMTPHost).Placeholder("smtp.example.com")
			}).Grow(1)
			formField(c, "Port", func() {
				ui.TextInput(c, &a.form.SMTPPort).Placeholder("587")
			}).Width(84)
		})
	})
}

func sectionLabel(c *ui.Context, text string) {
	t := c.Theme()
	ui.Text(c, strings.ToUpper(text)).FontSize(10).FontWeight(600).LetterSpacing(1.2).TextColor(t.TextMuted)
}

func formField(c *ui.Context, label string, input func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(t.TextMuted)
		input()
	})
}

func (a *App) applyPreset(p provider) {
	a.form.IMAPHost = p.imapHost
	a.form.IMAPPort = p.imapPort
	a.form.SMTPHost = p.smtpHost
	a.form.SMTPPort = p.smtpPort
	a.setupErr = ""
}

func (a *App) connect() {
	a.form.Email = strings.TrimSpace(a.form.Email)
	a.form.Name = strings.TrimSpace(a.form.Name)
	a.form.IMAPHost = strings.TrimSpace(a.form.IMAPHost)
	a.form.SMTPHost = strings.TrimSpace(a.form.SMTPHost)
	a.form.IMAPPort = strings.TrimSpace(a.form.IMAPPort)
	a.form.SMTPPort = strings.TrimSpace(a.form.SMTPPort)
	a.form.Password = strings.TrimSpace(a.form.Password)
	if a.form.IMAPHost == "" || a.form.SMTPHost == "" {
		if p, ok := guessProvider(a.form.Email); ok {
			a.applyPreset(p)
		}
	}
	if a.form.IMAPPort == "" {
		a.form.IMAPPort = "993"
	}
	if a.form.SMTPPort == "" {
		a.form.SMTPPort = "587"
	}
	if err := validateAccount(a.form); err != nil {
		a.setupErr = err.Error()
		a.connecting = false
		return
	}
	a.setupErr = ""
	a.busy = true
	acct := a.form
	go a.doConnect(acct)
}

func validateAccount(a Account) error {
	switch {
	case a.Email == "" || !strings.Contains(a.Email, "@"):
		return fmt.Errorf("Enter your full email address.")
	case a.Password == "":
		return fmt.Errorf("Enter your password — an app password for Gmail, iCloud or Yahoo.")
	case a.IMAPHost == "":
		return fmt.Errorf("Enter your IMAP server, for example imap.gmail.com.")
	case a.SMTPHost == "":
		return fmt.Errorf("Enter your SMTP server, for example smtp.gmail.com.")
	}
	return nil
}

func guessProvider(email string) (provider, bool) {
	domain := ""
	if i := strings.LastIndex(email, "@"); i >= 0 {
		domain = strings.ToLower(email[i+1:])
	}
	for _, p := range providers {
		switch p.name {
		case "Gmail":
			if domain == "gmail.com" || domain == "googlemail.com" {
				return p, true
			}
		case "Outlook":
			if strings.HasSuffix(domain, "outlook.com") || strings.HasSuffix(domain, "hotmail.com") || strings.HasSuffix(domain, "live.com") {
				return p, true
			}
		case "iCloud":
			if domain == "icloud.com" || domain == "me.com" || domain == "mac.com" {
				return p, true
			}
		case "Yahoo":
			if domain == "yahoo.com" || strings.HasSuffix(domain, ".yahoo.com") {
				return p, true
			}
		case "Fastmail":
			if strings.HasSuffix(domain, "fastmail.com") || strings.HasSuffix(domain, "fastmail.fm") {
				return p, true
			}
		}
	}
	return provider{}, false
}

func (a *App) doConnect(acct Account) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("doConnect", r)
		}
	}()
	if a.loadedLimit <= 0 {
		a.loadedLimit = 100
	}
	a.netMu.Lock()
	sess, folders, err := Open(acct)
	var msgs []Message
	if err == nil {
		msgs, err = sess.Messages("INBOX", 100, 0)
		if err != nil {
			sess.Close()
			sess = nil
		}
	}
	a.netMu.Unlock()

	a.mu.Lock()
	a.busy = false
	if err != nil {
		a.connecting = false
		if a.switching {
			a.switching = false
			a.active = a.switchFrom
			if a.active >= 0 && a.active < len(a.accounts) {
				a.form = a.accounts[a.active]
				a.applyAccountAppearance(a.form)
			}
			a.accountOpen = false
			a.errMsg = err.Error()
		} else {
			a.setupErr = err.Error()
		}
		a.mu.Unlock()
		a.requestWindowMaterials()
		a.invalidateAll()
		return
	}
	oldSess := a.sess
	a.sess = sess
	a.acct = acct
	a.applyAccountAppearance(acct)
	// Add a virtual "Flagged" entry if the provider has no such mailbox, before
	// applying any saved folder order so it lands in the right place.
	hasFlagged := false
	for _, f := range folders {
		if f.Kind == "flagged" {
			hasFlagged = true
			break
		}
	}
	if !hasFlagged {
		folders = append(folders, Folder{Name: "FLAGGED", Label: "Flagged", Kind: "flagged"})
	}
	if len(a.acct.FolderOrder) > 0 {
		byName := map[string]Folder{}
		for _, f := range folders {
			byName[f.Name] = f
		}
		matched := 0
		for _, name := range a.acct.FolderOrder {
			if _, ok := byName[name]; ok {
				matched++
			}
		}
		if matched >= len(folders)/2 || matched >= 3 {
			ordered := make([]Folder, 0, len(folders))
			seen := map[string]bool{}
			for _, name := range a.acct.FolderOrder {
				if f, ok := byName[name]; ok && !seen[name] {
					ordered = append(ordered, f)
					seen[name] = true
				}
			}
			for _, f := range folders {
				if !seen[f.Name] {
					ordered = append(ordered, f)
				}
			}
			folders = ordered
		} else {
			slices.SortStableFunc(folders, func(a, b Folder) int {
				if ra, rb := folderRank(a.Kind), folderRank(b.Kind); ra != rb {
					return ra - rb
				}
				return strings.Compare(a.Label, b.Label)
			})
		}
	} else {
		slices.SortStableFunc(folders, func(a, b Folder) int {
			if ra, rb := folderRank(a.Kind), folderRank(b.Kind); ra != rb {
				return ra - rb
			}
			return strings.Compare(a.Label, b.Label)
		})
	}
	a.folders = folders
	a.folder = "INBOX"
	a.sidebarSel = "INBOX"
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	a.viewingMsg = nil
	a.filterQuery = ""
	a.cache = map[string]*folderCache{}
	// Preload any locally cached mail for every folder so the UI is instant.
	if dc, ok := a.diskCache.Accounts[acct.Email]; ok && dc != nil {
		for _, f := range a.folders {
			if cached, ok := dc[f.Name]; ok && len(cached) > 0 {
				a.cache[f.Name] = &folderCache{
					msgs:        append([]Message(nil), cached...),
					loadedLimit: len(cached),
					hasMore:     true,
					sel:         firstUnread(cached),
					scrollY:     0,
				}
			}
		}
	}
	// Promote the INBOX cache to the active list.
	if ce, ok := a.cache["INBOX"]; ok && ce != nil {
		a.msgs = ce.msgs
		a.loadedLimit = ce.loadedLimit
		a.hasMore = ce.hasMore
		a.sel = ce.sel
		delete(a.cache, "INBOX")
	} else {
		a.msgs = nil
		a.loadedLimit = 0
		a.hasMore = true
	}
	// Merge the freshly fetched first page into the cached list.
	if err == nil {
		existing := map[imap.UID]bool{}
		for _, m := range a.msgs {
			existing[m.UID] = true
		}
		for _, nm := range msgs {
			for i := range a.msgs {
				if a.msgs[i].UID == nm.UID {
					a.msgs[i].Read = nm.Read
					a.msgs[i].Flagged = nm.Flagged
				}
			}
		}
		var newMsgs []Message
		for _, nm := range msgs {
			if !existing[nm.UID] {
				newMsgs = append(newMsgs, nm)
				existing[nm.UID] = true
			}
		}
		a.msgs = append(newMsgs, a.msgs...)
		a.loadedLimit = len(a.msgs)
		if len(msgs) < 100 && a.loadedLimit <= 100 {
			a.hasMore = false
		} else {
			a.hasMore = true
		}
		a.sel = firstUnread(a.msgs)
	}
	a.body = ""
	a.bodyUID = 0
	a.showCompose = false
	a.note = ""
	a.connected = true
	a.connecting = false
	a.switching = false
	a.accountOpen = false
	a.connID++
	a.bgFetching = true
	a.fetchGen++
	gen := a.fetchGen
	go a.fetchAllMailboxes(gen)
	found := -1
	for i, acc := range a.accounts {
		if acc.Email == acct.Email {
			found = i
			break
		}
	}
	if found >= 0 {
		a.accounts[found] = acct
		a.active = found
	} else {
		a.accounts = append(a.accounts, acct)
		a.active = len(a.accounts) - 1
	}
	a.saveConfig()
	uid := imap.UID(0)
	if a.sel >= 0 && a.sel < len(a.msgs) {
		uid = a.msgs[a.sel].UID
	}
	a.mu.Unlock()
	a.startKeepAlive()
	a.saveDiskCache()
	a.requestWindowMaterials()
	a.invalidateAll()
	if oldSess != nil && oldSess != sess {
		go func() {
			a.netMu.Lock()
			oldSess.Close()
			a.netMu.Unlock()
		}()
	}
	if uid != 0 {
		go a.loadBody(uid)
	}
}

// ----------------------------------------------------------------- mail ---

func (a *App) mailView(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		if a.sidebarAnimW > 1 {
			a.sidebar(c)
		}
		main := ui.Column(c).Grow(1).MinWidth(0).HeightPercent(100).Background(ui.Transparent)
		main.Children(func() {
			stage := ui.Column(c).Grow(1).MinHeight(0).Margin(0, 4, 4, 4).
				Background(paneColor(t)).Radius(18).Border(1, t.Border).Clip()
			stage.Children(func() {
				ui.Column(c).Fill().Padding(22, 24, 28, 24).Children(func() {
					ui.Split(c, &a.split, func() { a.listPane(c) }, func() { a.reader(c) }).Grow(1).FillWidth().MinHeight(0)
				})
			})
		})
	})
}

func (a *App) splitWithHiddenDivider(c *ui.Context, p palette, vibrancy bool, size *float32, first, second func()) ui.Element {
	if !vibrancy && !p.HasGradient {
		return ui.Split(c, size, first, second)
	}
	appTheme := c.Theme()
	dividerTheme := *appTheme
	dividerTheme.Border = ui.Transparent
	c.SetTheme(&dividerTheme)
	restore := func() {
		c.SetTheme(appTheme)
		a.paintChrome(c)
	}
	split := ui.Split(c, size, func() {
		restore()
		first()
	}, func() {
		restore()
		second()
	})
	restore()
	return split
}

func hideSplitDivider(e ui.Element, size *float32, bg ui.Color, minPane float32) {
	e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 {
			return
		}
		x := *size
		upper := r.W - 1 - minPane
		if upper < minPane {
			upper = minPane
		}
		if x < minPane {
			x = minPane
		}
		if x > upper {
			x = upper
		}
		p.Fill(ui.Rect{X: x, Y: 0, W: 1, H: r.H}, bg, 0)
	})
}
func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(a.sidebarAnimW).Shrink(0).FillHeight().Padding(10).Gap(8).Children(func() {
		accountBtn := ui.ButtonBase(c.Key("accountMenu")).FillWidth().Padding(8, 9).Radius(10).Background(t.Surface).Border(1, t.Border)
		accountBtn.Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Avatar(c, a.displayName(), nil).Size(30, 30)
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, "ACCOUNT").FontSize(9).FontWeight(600).LetterSpacing(1.1).TextColor(t.TextMuted)
					ui.Text(c, a.acct.Email).FontSize(12.5).FontWeight(500).SingleLine()
				})
				if a.connecting {
					loading.Ring(c, loading.Options{Size: 14, Color: t.Accent})
				} else {
					// One chevron that flips when the menu opens, like Go Post's
					// workspace switcher, instead of a static up/down pair.
					chev := ui.Icon(c, iconChevronDown).Size(13, 13).TextColor(t.TextMuted)
					target := float32(0)
					if a.accountOpen {
						target = 180
					}
					chev.Rotate(chev.Animate("account-chevron", target, 160*time.Millisecond))
				}
			})
		})
		if accountBtn.Hovered() {
			accountBtn.Background(t.SurfaceHover)
		}
		if accountBtn.Clicked() && !a.connecting {
			a.accountOpen = !a.accountOpen
		}
		menuTarget := float32(0)
		if a.accountOpen {
			menuTarget = 1
		}
		menuAnim := accountBtn.Animate("accountMenuAnim", menuTarget, 140*time.Millisecond)
		ui.PopoverBase(c, accountBtn, &a.accountOpen, func(panel ui.Element) {
			panel.MinWidth(accountBtn.Bounds().W).Background(t.Surface).Border(1, t.Border).Radius(10).
				Padding(4).Shadow(0, 12, 30, 0, ui.RGBA(0, 0, 0, 0.28)).
				Opacity(menuAnim).Translate(0, (1-menuAnim)*-6)
			panel.Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for i, acc := range a.accounts {
						i := i
						acc := acc
						row := ui.ButtonBase(c.Key(fmt.Sprintf("acct:%d", i))).FillWidth().Padding(7, 9).Radius(8)
						row.Children(func() {
							ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
								ui.Avatar(c, acc.Name, nil).Size(24, 24)
								ui.Text(c, acc.Email).FontSize(12.5).Grow(1).SingleLine()
								if a.connecting && i == a.active {
									loading.Ring(c, loading.Options{Size: 14, Color: t.Accent})
								} else if i == a.active {
									ui.Icon(c, iconCheck).FontSize(13).TextColor(t.Accent)
								}
							})
						})
						if row.Hovered() && !a.connecting {
							row.Background(t.SurfaceHover)
						}
						if row.Clicked() && !a.connecting {
							if i != a.active {
								// Keep the switcher open so the loading animation
								// stays visible while the account connects.
								a.accountOpen = true
								go a.switchAccount(i)
							} else {
								a.accountOpen = false
							}
						}
					}
					add := ui.ButtonBase(c.Key("addAccount")).FillWidth().Padding(7, 9).Radius(8)
					add.Children(func() {
						ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconPlus).FontSize(14).TextColor(t.Accent)
							ui.Text(c, "Add account").FontSize(12.5).TextColor(t.Accent)
						})
					})
					if add.Hovered() {
						add.Background(t.SurfaceHover)
					}
					if add.Clicked() {
						a.accountOpen = false
						go a.startAddAccount()
					}
				})
			})
		})

		compose := ui.PrimaryButton(c.Key("compose"), "").Padding(9, 12).Radius(9)
		compose.Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconPen).FontSize(14).TextColor(t.AccentText)
				ui.Text(c, "Compose").FontSize(12.5).FontWeight(600).TextColor(t.AccentText)
			})
		})
		if compose.Clicked() {
			a.startCompose()
		}

		ui.Text(c, "MAILBOXES").FontSize(10).FontWeight(600).LetterSpacing(1.2).
			TextColor(t.TextMuted).Padding(8, 8, 2, 8)

		// Folders are dragged like Go Post's request tree rows: each row is its
		// own drop target, a line shows which half it will land on, and the
		// rows glide to their new place after the drop.
		ui.Scroll(c).TrackScroll(&a.folderScrollState).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(1).Children(func() {
				var pending folderDrag
				pendingAt, hasDrop := 0, false
				for i := range a.folders {
					a.folderRow(c, a.folders[i], i, &pending, &pendingAt, &hasDrop)
				}
				// A drop changes a.folders, so apply it after the rows are built.
				if hasDrop {
					a.reorderFolder(pending.Name, pendingAt)
				}
			})
		})
		if a.sharedMailbox {
			a.folderRow(c, Folder{Name: "SHARED", Label: "Shared Mailbox", Kind: "shared"}, -1, nil, nil, nil)
		}

		if a.note != "" {
			ui.Text(c, a.note).FontSize(11).TextColor(t.TextMuted).Wrap().Padding(4, 8)
		}

		searchBtn := ui.ButtonBase(c.Key("search")).FillWidth().Padding(7, 9).Radius(8)
		searchBtn.Children(func() {
			ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconSearch).FontSize(15).TextColor(t.TextMuted)
				ui.Text(c, "Command").FontSize(12.5).TextColor(t.TextMuted).Grow(1)
				ui.Text(c, "Ctrl+Space").FontSize(9.5).TextColor(t.TextMuted)
			})
		})
		if searchBtn.Hovered() {
			searchBtn.Background(t.SurfaceHover)
		}
		if searchBtn.Clicked() {
			a.searchOpen = true
			a.searchQuery = ""
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			settingsBtn := ui.ButtonBase(c.Key("settings")).Padding(6, 8).Radius(8).Grow(1)
			settingsBtn.Children(func() {
				ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconGear).FontSize(14).TextColor(t.TextMuted)
					ui.Text(c, "Settings").FontSize(12).TextColor(t.TextMuted).Grow(1).SingleLine()
					ui.Text(c, "Ctrl+T").FontSize(9.5).TextColor(t.TextMuted)
				})
			})
			if settingsBtn.Hovered() {
				settingsBtn.Background(t.SurfaceHover)
			}
			if settingsBtn.Clicked() {
				a.openSettings()
			}
			if iconButton(c, "logout", iconLogout).Clicked() {
				a.logout()
			}
		})
	})
}

// folderDrag identifies a mailbox row being dragged in the sidebar.
type folderDrag struct {
	Name string
}

// folderRowTransition makes rows glide to their new place after a drop instead
// of jumping, while the drop highlight eases in on the colors only.
var folderRowTransition = ui.ElementTransition{
	Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 220 * time.Millisecond}},
	Position:  true,
}

func (a *App) folderRow(c *ui.Context, f Folder, pos int, pending *folderDrag, pendingAt *int, hasDrop *bool) {
	t := c.Theme()
	chosen := f.Name == a.sidebarSel
	iconColor := t.TextMuted
	weight := 500
	if chosen {
		iconColor = t.Text
		weight = 600
	}
	row := ui.ButtonBase(c.Key("folder:"+f.Name)).FillWidth().Padding(7, 9).Radius(8).
		Label(f.Label).Transition(folderRowTransition)
	if pos >= 0 {
		row.Drag(folderDrag{Name: f.Name})

		// The row is its own drop target. Which half the pointer is over
		// decides whether the folder lands above or below it.
		insert := -1
		if v, ok := ui.DragOver[folderDrag](row); ok && v.Name != f.Name {
			bounds := row.Bounds()
			_, py, over := row.PointerPosition()
			if over && bounds.H > 0 && py >= bounds.H/2 {
				insert = pos + 1
			} else {
				insert = pos
			}
		}
		after := ui.Local(row, "folderDropAfter", func() bool { return false })
		if insert >= 0 {
			*after = insert == pos+1
		}
		lineTarget := float32(0)
		if insert >= 0 {
			lineTarget = 1
		}
		line := row.Animate("folderDropLine", lineTarget, 140*time.Millisecond)
		row.DrawOver(func(pt *ui.Painter, box ui.Rect) {
			if line <= 0.01 {
				return
			}
			y := box.Y
			if *after {
				y = box.Y + box.H - 2
			}
			pt.Fill(ui.Rect{X: box.X + 4, Y: y, W: box.W - 8, H: 2}, t.Accent.Alpha(line), 1)
		})
		if v, ok := ui.Drop[folderDrag](row); ok && !*hasDrop && v.Name != f.Name {
			*pending = v
			*pendingAt = pos
			if _, py, over := row.PointerPosition(); over && row.Bounds().H > 0 && py >= row.Bounds().H/2 {
				*pendingAt = pos + 1
			}
			*hasDrop = true
		}
	}
	row.Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, folderIcon(f.Kind)).FontSize(15).TextColor(iconColor)
			ui.Text(c, f.Label).FontSize(13).FontWeight(weight).TextColor(t.Text).Grow(1).SingleLine()
			if f.Unread > 0 {
				ui.Text(c, fmt.Sprint(f.Unread)).FontSize(11).FontWeight(600).TextColor(t.Accent)
			}
		})
	})
	if chosen {
		row.Background(t.Surface)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		a.sidebarSel = f.Name
		go a.selectFolder(f.Name)
	}
}

// reorderFolder moves a dragged mailbox to a position in the sidebar and saves
// the order for this account. before is the index it should land at.
func (a *App) reorderFolder(name string, before int) {
	from := -1
	for i := range a.folders {
		if a.folders[i].Name == name {
			from = i
			break
		}
	}
	if from < 0 || name == "SHARED" {
		return
	}
	f := a.folders[from]
	a.folders = append(a.folders[:from], a.folders[from+1:]...)
	if before > from {
		before--
	}
	if before < 0 || before > len(a.folders) {
		before = len(a.folders)
	}
	a.folders = append(a.folders[:before], append([]Folder{f}, a.folders[before:]...)...)
	a.acct.FolderOrder = make([]string, len(a.folders))
	for i, folder := range a.folders {
		a.acct.FolderOrder[i] = folder.Name
	}
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active] = a.acct
	}
	a.saveConfig()
	a.invalidateAll()
}

type listEntry struct {
	index int
	msg   Message
	ext   bool
}

func (a *App) activeFilterCount() int {
	n := 0
	if a.filterFlagged {
		n++
	}
	if a.filterUnread {
		n++
	}
	if strings.TrimSpace(a.filterFrom) != "" {
		n++
	}
	if strings.TrimSpace(a.filterTo) != "" {
		n++
	}
	if strings.TrimSpace(a.filterDateFrom) != "" {
		n++
	}
	if strings.TrimSpace(a.filterDateTo) != "" {
		n++
	}
	return n
}

func mailMatches(m Message, q string) bool {
	return strings.Contains(strings.ToLower(m.Subject+" "+m.FromName+" "+m.FromAddr), q)
}

func (a *App) messagePassesFilters(m Message) bool {
	if a.filterFlagged && !m.Flagged {
		return false
	}
	if a.filterUnread && m.Read {
		return false
	}
	if f := strings.TrimSpace(a.filterFrom); f != "" {
		if !strings.Contains(strings.ToLower(m.FromName+" "+m.FromAddr), strings.ToLower(f)) {
			return false
		}
	}
	if f := strings.TrimSpace(a.filterTo); f != "" {
		if !strings.Contains(strings.ToLower(strings.Join(m.To, " ")), strings.ToLower(f)) {
			return false
		}
	}
	if d := strings.TrimSpace(a.filterDateFrom); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			if m.Date.Before(t) {
				return false
			}
		}
	}
	if d := strings.TrimSpace(a.filterDateTo); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			end := t.Add(24 * time.Hour)
			if !m.Date.Before(end) {
				return false
			}
		}
	}
	return true
}

// resetSearch clears the folder search and resets state.
func (a *App) resetSearch() {
	a.filterQuery = ""
	a.invalidate()
}

// requestSearch sets the search query; listPane filters a.msgs locally.
// No server search is performed to avoid breaking the app state.
// requestSearch sets the search query; listPane filters a.msgs locally.
// No server search is performed to avoid breaking the app state.
func (a *App) requestSearch(q string) {
	a.filterQuery = q
	a.invalidate()
}

func (a *App) listPane(c *ui.Context) {
	t := c.Theme()
	q := strings.ToLower(strings.TrimSpace(a.filterQuery))
	var entries []listEntry
	for i, m := range a.msgs {
		if !a.messagePassesFilters(m) {
			continue
		}
		if q == "" {
			entries = append(entries, listEntry{index: i, msg: m})
		} else if mailMatches(m, a.filterQuery) {
			entries = append(entries, listEntry{index: -1, msg: m, ext: true})
		}
	}
	// Switching mailboxes swaps the list under a short fade and rise, like
	// Go Post's page transitions, so the new folder does not teleport in.
	ui.Column(c).Key("mail-list:" + a.folder).Fill().Transition(ui.ElementTransition{
		Duration: 170 * time.Millisecond,
		Enter:    &ui.Motion{Y: 5, Opacity: 0},
	}).Children(func() {
		ui.Row(c).Padding(10, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.folderLabel()).FontSize(16).FontWeight(700).Grow(1).SingleLine()
			if a.loadingMsgs {
				ui.Text(c, "Loading…").FontSize(11.5).TextColor(t.TextMuted)
			} else if a.bgFetching {
				ui.Text(c, fmt.Sprintf("%d (loading more…)", len(entries))).FontSize(11.5).TextColor(t.TextMuted)
			} else {
				ui.Text(c, fmt.Sprintf("%d", len(entries))).FontSize(11.5).TextColor(t.TextMuted)
			}
			if iconButton(c, "refreshList", iconRefresh).Clicked() {
				a.refresh()
			}
		})
		// Search bar: matches only the open mailbox.
		ui.Row(c).Padding(0, 12, 8, 12).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.TextInput(c, &a.filterQuery).Placeholder("Search this mailbox…").Height(30).Grow(1).FontSize(12).Label("Search mail")
			if a.filterQuery != "" {
				if iconButton(c, "clearFilter", iconX).Clicked() {
					a.resetSearch()
				}
			}
			filterWrap := ui.Box(c).Size(40, 40).Shrink(0)
			var filterBtn ui.Element
			filterWrap.Children(func() {
				filterBtn = ui.ButtonBase(c.Key("filterToggle")).Size(40, 40).Radius(20).Center()
				filterBtn.Children(func() {
					ui.Icon(c, iconFilter).FontSize(22).TextColor(t.TextMuted)
				})
				if filterBtn.Hovered() {
					filterBtn.Background(t.SurfaceHover)
				}
				if filterBtn.Clicked() {
					a.filterPanelOpen = !a.filterPanelOpen
				}
				if n := a.activeFilterCount(); n > 0 {
					w := float32(18)
					if n > 9 {
						w = float32(22)
					}
					badge := ui.Box(c).Absolute().Top(-2).Left(40-w+2).Size(w, w).Radius(w / 2).Background(t.Accent).Center().Shrink(0)
					badge.Children(func() {
						ui.Text(c, fmt.Sprintf("%d", n)).FontSize(10).FontWeight(700).TextColor(ui.RGB(255, 255, 255))
					})
					_ = badge
				}
			})
			a.filterDropdown(c, t, filterBtn)
		})
		ui.Divider(c)
		if a.errMsg != "" {
			ui.Box(c).FillWidth().Padding(8, 14).Background(t.Danger.Alpha(0.1)).Children(func() {
				ui.Text(c, a.errMsg).FontSize(11.5).TextColor(t.Danger).Wrap()
			})
		}
		if a.loadingMsgs && len(a.msgs) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Padding(24).Children(func() {
				ui.Text(c, "Loading…").FontSize(12.5).TextColor(t.TextMuted)
			})
			return
		}
		if len(a.msgs) == 0 && len(entries) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Padding(24).Children(func() {
				ui.Box(c).Size(44, 44).Radius(22).Background(t.Surface).Center().Children(func() {
					ui.Icon(c, iconInbox).FontSize(20).TextColor(t.TextMuted)
				})
				ui.Text(c, "Nothing here yet").FontSize(14).FontWeight(600)
				ui.Text(c, "Messages in this folder will show up here.").FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center)
			})
			return
		}
		if len(entries) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Padding(24).Children(func() {
				if q != "" {
					ui.Text(c, "No matching mail").FontSize(14).FontWeight(600)
					ui.Text(c, "Try a different search.").FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center)
				}
			})
			return
		}
		ui.Scroll(c).TrackScroll(&a.scrollState).Grow(1).Children(func() {
			ui.Column(c).Padding(6).Gap(1).Children(func() {
				const rowH = 72.0
				total := len(entries)
				if total == 0 {
					return
				}
				// Handle pending scroll (e.g., from search).
				if a.pendingScroll {
					for pos, e := range entries {
						if e.index == a.sel && e.index >= 0 {
							a.scrollState.Y = float32(pos) * rowH
							break
						}
					}
					a.pendingScroll = false
				}
				scrollY := float64(a.scrollState.Y)
				start := int(scrollY/rowH) - 4
				if start < 0 {
					start = 0
				}
				// Estimate viewport height ~700px.
				visible := 12
				end := start + visible + 8
				if end > total {
					end = total
				}
				if start > end {
					start = end
				}
				if start > 0 {
					ui.Box(c).Height(float32(start) * rowH).Shrink(0)
				}
				for pos := start; pos < end; pos++ {
					e := entries[pos]
					if e.ext {
						a.searchResultRow(c, e.msg)
					} else {
						a.messageRow(c, e.index)
					}
				}
				if end < total {
					ui.Box(c).Height(float32(total-end) * rowH).Shrink(0)
				}
			})
		})
	})
}

func (a *App) messageRow(c *ui.Context, i int) {
	if i < 0 || i >= len(a.msgs) {
		return
	}
	t := c.Theme()
	m := a.msgs[i]
	chosen := i == a.sel
	nameColor := t.Text
	subjectColor := t.TextMuted
	weight := 500
	if !m.Read {
		subjectColor = t.Text
		weight = 600
	}
	row := ui.ButtonBase(c.Key(fmt.Sprintf("msg:%d:%s", m.UID, m.Folder))).FillWidth().Padding(10).Radius(10)
	if row.RightClicked() {
		a.openMessageMenu(row, i)
	}
	row.Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			a.senderAvatar(c, m.FromName, m.FromAddr, 32)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 10).Children(func() {
					if a.folder == "SHARED" {
						ui.Box(c).Size(8, 8).Radius(4).Background(a.accountColor(m.Folder)).Shrink(0)
						ui.Text(c, m.Folder).FontSize(10).FontWeight(600).TextColor(a.accountColor(m.Folder)).Shrink(0).SingleLine()
					}
					ui.Text(c, m.FromName).FontSize(12.5).FontWeight(weight).TextColor(nameColor).Grow(1).SingleLine()
					ui.Text(c, shortDate(m.Date)).FontSize(10.5).TextColor(t.TextMuted).Shrink(0)
					if m.Flagged {
						ui.Icon(c, iconFlag).FontSize(12).TextColor(t.Warning).Shrink(0)
					}
					if len(m.Attachments) > 0 {
						ui.Icon(c, iconAttachment).FontSize(12).TextColor(t.TextMuted).Shrink(0)
					}
					if !m.Read {
						ui.Box(c).Size(6, 6).Radius(3).Background(t.Accent).Shrink(0)
					}
				})
				ui.Text(c, m.Subject).FontSize(12.5).TextColor(subjectColor).SingleLine()
			})
		})
	})
	if chosen {
		row.Background(t.Surface)
		if a.pendingScroll {
			row.ScrollIntoView()
			a.pendingScroll = false
		}
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if m.Flagged && !chosen {
		row.Border(1, t.Warning)
	}
	if row.Clicked() {
		a.selectMessage(i)
	}

}

func (a *App) searchResultRow(c *ui.Context, m Message) {
	t := c.Theme()
	nameColor := t.Text
	subjectColor := t.TextMuted
	weight := 500
	if !m.Read {
		subjectColor = t.Text
		weight = 600
	}
	chosen := a.viewingMsg != nil && a.viewingMsg.UID == m.UID && a.viewingFolder == m.Folder
	row := ui.ButtonBase(c.Key(fmt.Sprintf("ext:%s:%d", m.Folder, m.UID))).FillWidth().Padding(10).Radius(10)
	row.Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			a.senderAvatar(c, m.FromName, m.FromAddr, 32)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 10).Children(func() {
					if a.folder == "SHARED" {
						ui.Box(c).Size(8, 8).Radius(4).Background(a.accountColor(m.Folder)).Shrink(0)
						ui.Text(c, m.Folder).FontSize(10).FontWeight(600).TextColor(a.accountColor(m.Folder)).Shrink(0).SingleLine()
					}
					ui.Text(c, m.FromName).FontSize(12.5).FontWeight(weight).TextColor(nameColor).Grow(1).SingleLine()
					ui.Text(c, shortDate(m.Date)).FontSize(10.5).TextColor(t.TextMuted).Shrink(0)
					if m.Flagged {
						ui.Icon(c, iconFlag).FontSize(12).TextColor(t.Warning).Shrink(0)
					}
					if !m.Read {
						ui.Box(c).Size(6, 6).Radius(3).Background(t.Accent).Shrink(0)
					}
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, m.Subject).FontSize(12.5).TextColor(subjectColor).Grow(1).SingleLine()
					ui.Text(c, m.Folder).FontSize(9).TextColor(t.Accent).Shrink(0)
				})
			})
		})
	})
	if chosen {
		row.Background(t.Surface)
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if m.Flagged && !chosen {
		row.Border(1, t.Warning)
	}
	if row.Clicked() {
		a.viewMessage(m)
	}
}

func (a *App) selectMessage(i int) {
	if i < 0 || i >= len(a.msgs) {
		return
	}
	a.sel = i
	a.viewingMsg = nil
	a.showCompose = false
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	uid := a.msgs[i].UID
	if a.folder == "SHARED" || strings.Contains(a.msgs[i].Folder, "@") {
		go a.loadSharedBody(a.msgs[i])
	} else {
		go a.loadBody(uid)
	}
}

// viewMessage opens a message from search without loading or scrolling its
// folder: it fetches just that message and shows it in the reader.
func (a *App) viewMessage(m Message) {
	a.viewingMsg = &m
	a.viewingFolder = m.Folder
	a.viewingUID = m.UID
	a.sel = -1
	a.showCompose = false
	a.body = ""
	a.bodyHTML = ""
	a.bodyLoading = true
	a.bodyUID = m.UID
	a.viewAsText = false
	a.invalidateAll()
	if a.folder == "SHARED" || strings.Contains(m.Folder, "@") {
		go a.loadSharedBody(m)
	} else {
		folder := m.Folder
		if folder == "" {
			folder = a.folder
		}
		if folder == "" || folder == "SHARED" {
			folder = "INBOX"
		}
		go a.loadBodyFrom(folder, m.UID)
	}
}

// currentTarget is the message the reader shows and where it lives. index is
// -1 when the message was opened from search and is not in the loaded list.
func (a *App) currentTarget() (m Message, folder string, uid imap.UID, index int, ok bool) {
	if a.viewingMsg != nil {
		return *a.viewingMsg, a.viewingFolder, a.viewingUID, -1, true
	}
	if a.sel >= 0 && a.sel < len(a.msgs) {
		return a.msgs[a.sel], a.folder, a.msgs[a.sel].UID, a.sel, true
	}
	return
}

func (a *App) reader(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		switch {
		case a.showCompose:
			a.composeView(c)
		case a.viewingMsg != nil:
			a.messageView(c, *a.viewingMsg)
		case a.sel >= 0 && a.sel < len(a.msgs):
			a.messageView(c, a.msgs[a.sel])
		default:
			a.emptyView(c)
		}
	})
}

func (a *App) messageView(c *ui.Context, m Message) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 14).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			if actionButton(c, "reply", iconReply, "Reply").Clicked() {
				a.reply()
			}
			if actionButton(c, "replyAll", iconReply, "Reply all").Clicked() {
				a.replyAll()
			}
			if actionButton(c, "forward", iconSent, "Forward").Clicked() {
				a.forward()
			}
			if a.themeSettings.Debug {
				label := "View as text"
				if a.viewAsText {
					label = "View as HTML"
				}
				if actionButton(c, "toggleText", iconEye, label).Clicked() {
					a.viewAsText = !a.viewAsText
				}
			}
		})
		ui.Divider(c).Shrink(0)
		if a.bodyLoading && a.bodyUID == m.UID {
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, "Loading message…").FontSize(13).TextColor(t.TextMuted)
			})
			return
		}
		if a.bodyHTML != "" && a.readerView != nil && !a.viewAsText {
			ui.Column(c).Padding(12, 20).Shrink(0).Children(func() {
				ui.Text(c, m.Subject).FontSize(21).FontWeight(700).LineHeight(1.3)
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					a.senderAvatar(c, m.FromName, m.FromAddr, 38)
					ui.Column(c).Gap(1).MinWidth(0).Children(func() {
						ui.Text(c, m.FromName).FontSize(13.5).FontWeight(600).SingleLine()
						ui.Text(c, m.FromAddr).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
						if a.folder == "SHARED" {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(8, 8).Radius(4).Background(a.accountColor(m.Folder))
								ui.Text(c, m.Folder).FontSize(11.5).TextColor(a.accountColor(m.Folder)).SingleLine()
							})
						}
						if len(m.To) > 0 {
							ui.Text(c, "To: "+strings.Join(m.To, ", ")).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
						}
						ui.Text(c, m.Date.Format("Mon, 2 Jan 2006 at 15:04")).FontSize(11).TextColor(t.TextMuted)
					})
				})
			})
			ui.Divider(c).Shrink(0)
			ui.WebView(c, a.readerView).Grow(1).MinHeight(100)
			a.attachmentSection(c, m, t)
		} else {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(26, 32).Gap(16).MaxWidth(760).Children(func() {
					ui.Text(c, m.Subject).FontSize(21).FontWeight(700).LineHeight(1.3)
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						a.senderAvatar(c, m.FromName, m.FromAddr, 38)
						ui.Column(c).Gap(1).MinWidth(0).Children(func() {
							ui.Text(c, m.FromName).FontSize(13.5).FontWeight(600).SingleLine()
							ui.Text(c, m.FromAddr).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
							if a.folder == "SHARED" {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Box(c).Size(8, 8).Radius(4).Background(a.accountColor(m.Folder))
									ui.Text(c, m.Folder).FontSize(11.5).TextColor(a.accountColor(m.Folder)).SingleLine()
								})
							}
							if len(m.To) > 0 {
								ui.Text(c, "To: "+strings.Join(m.To, ", ")).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
							}
							ui.Text(c, m.Date.Format("Mon, 2 Jan 2006 at 15:04")).FontSize(11).TextColor(t.TextMuted)
						})
					})
					ui.Divider(c)
					if a.body == "" {
						ui.Text(c, "(This message has no text content.)").FontSize(13).TextColor(t.TextMuted)
					} else {
						ui.Text(c, a.body).FontSize(13.5).LineHeight(1.65).TextColor(bodyColor(t)).Selectable()
					}
					a.attachmentSection(c, m, t)
				})
			})
		}
	})
}

// attachmentSection renders the files of a message. The heading and the rows
// share the same inset so nothing hugs the left edge, and each file sits in a
// quiet card with its actions on the right.
func (a *App) attachmentSection(c *ui.Context, m Message, t *ui.Theme) {
	if len(m.Attachments) == 0 {
		return
	}
	ui.Divider(c).Shrink(0)
	ui.Column(c).FillWidth().Padding(10, 12, 4, 12).Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconAttachment).FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "Attachments").FontSize(12).FontWeight(600).Grow(1)
			ui.Text(c, fmt.Sprintf("%d", len(m.Attachments))).FontSize(10).FontWeight(600).
				TextColor(t.TextMuted).Padding(2, 7).Radius(99).Background(t.Surface)
		})
		for _, att := range m.Attachments {
			att := att
			row := ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Padding(8, 10).Radius(9).
				Background(t.Surface).Border(1, t.Border)
			if row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				ui.Box(c).Size(30, 30).Radius(8).Background(t.SurfaceHover).Center().Children(func() {
					ui.Icon(c, iconAttachment).FontSize(14).TextColor(t.TextMuted)
				})
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, att.Name).FontSize(11.5).FontWeight(560).SingleLine().Ellipsis(att.Name)
					ui.Text(c, formatBytes(int64(att.Size))).FontSize(9.5).TextColor(t.TextMuted)
				})
				if ui.Button(c.Key("preview:"+att.Name), "Preview").Padding(4, 10).Radius(6).Clicked() {
					a.previewAttachment(m, att)
				}
				if ui.Button(c.Key("dl:"+att.Name), "Download").Padding(4, 10).Radius(6).Clicked() {
					a.downloadAttachment(m, att)
				}
			})
		}
	})
}

func (a *App) emptyView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(10).Padding(40).Children(func() {
		ui.Box(c).Size(56, 56).Radius(28).Background(t.Surface).Center().Children(func() {
			ui.Icon(c, iconMail).FontSize(24).TextColor(t.TextMuted)
		})
		ui.Text(c, "Select a message").FontSize(15).FontWeight(600)
		ui.Text(c, "Pick a conversation on the left to read it here.").FontSize(12.5).TextColor(t.TextMuted).TextAlign(ui.Center)
	})
}

func (a *App) composeView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20, 24).Gap(12).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "New message").FontSize(17).FontWeight(700).Grow(1)
			if iconButton(c, "closeCompose", iconX).Clicked() {
				a.closeCompose()
			}
		})
		formField(c, "To", func() {
			ui.TextInput(c, &a.composeTo).Placeholder("name@example.com")
		})
		formField(c, "Cc", func() {
			ui.TextInput(c, &a.composeCc).Placeholder("name@example.com (optional)")
		})
		formField(c, "Subject", func() {
			ui.TextInput(c, &a.composeSub).Placeholder("Subject")
		})
		ui.Column(c).Grow(1).Gap(6).Children(func() {
			a.composeToolbar(c)
			doc := a.composeEditorDoc()
			// The editor draws its own surface and border, so it is the
			// only input: no box inside a box.
			richtext.Editor(c, doc, richtext.EditorOptions{
				Label:       "Message",
				Placeholder: "Write your message…",
				Font:        ui.Font{Size: 14},
				NoMarkdown:  true,
				Theme:       signatureRichTheme(c, a.palette),
			}).Grow(1).MinHeight(180)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if actionButton(c, "attachFiles", iconAttachment, "Attach files").Clicked() {
				a.pickAttachments()
			}
		})
		if len(a.composeAttachments) > 0 {
			ui.Column(c).Gap(4).Children(func() {
				for i, att := range a.composeAttachments {
					i, att := i, att
					ui.Row(c).FillWidth().Padding(6, 10).Radius(8).Gap(8).
						AlignItems(ui.Center).Background(t.Surface).Children(func() {
						ui.Icon(c, iconAttachment).FontSize(14).TextColor(t.TextMuted)
						ui.Text(c, att.Name).FontSize(12).Grow(1).SingleLine()
						ui.Text(c, formatBytes(att.Size)).FontSize(10).TextColor(t.TextMuted)
						if iconButton(c, fmt.Sprintf("removeAtt:%d", i), iconX).Clicked() {
							a.composeAttachments = append(a.composeAttachments[:i], a.composeAttachments[i+1:]...)
						}
					})
				}
			})
		}
		if a.composeErr != "" {
			ui.Box(c).FillWidth().Padding(9, 14).Radius(9).Background(t.Danger.Alpha(0.1)).Children(func() {
				ui.Text(c, a.composeErr).FontSize(12).TextColor(t.Danger)
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 10).Children(func() {
			if a.sending {
				ui.Text(c, "Sending…").FontSize(12).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if ui.Button(c.Key("cancelSend"), "Cancel").Padding(7, 14).Radius(8).Clicked() {
				a.closeCompose()
			}
			send := ui.PrimaryButton(c.Key("send"), "Send").Padding(8, 24).Radius(8)
			if a.sending {
				send.Disabled(true)
			}
			if send.Clicked() && !a.sending {
				a.send()
			}
		})
	})
}

func (a *App) startCompose() {
	a.showCompose = true
	a.composeTo = ""
	a.composeCc = ""
	a.composeSub = ""
	a.composeBody = ""
	a.composeQuote = ""
	a.composeAttachments = nil
	a.composeErr = ""
	a.startComposeDoc()
}

func (a *App) replyAll() {
	m, _, _, _, ok := a.currentTarget()
	if !ok {
		return
	}
	subject := m.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	recipients := []string{m.FromAddr}
	recipients = append(recipients, m.To...)
	recipients = append(recipients, m.Cc...)
	// dedupe
	seen := map[string]bool{}
	var unique []string
	for _, r := range recipients {
		if r == "" || r == a.acct.Email || seen[r] {
			continue
		}
		seen[r] = true
		unique = append(unique, r)
	}
	a.showCompose = true
	a.composeTo = strings.Join(unique, ", ")
	a.composeCc = ""
	a.composeSub = subject
	a.composeBody = ""
	a.composeQuote = "\n\nOn " + m.Date.Format("Mon, 2 Jan 2006 at 15:04") + ", " + m.FromName + " wrote:\n" + a.body
	a.composeAttachments = nil
	a.startComposeDoc()
}

func (a *App) forward() {
	m, _, _, _, ok := a.currentTarget()
	if !ok {
		return
	}
	subject := m.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}
	a.showCompose = true
	a.composeTo = ""
	a.composeCc = ""
	a.composeSub = subject
	a.composeBody = ""
	a.composeQuote = "\n\n---------- Forwarded message ----------\nFrom: " + m.FromName + " <" + m.FromAddr + ">\nDate: " + m.Date.Format("Mon, 2 Jan 2006 at 15:04") + "\nSubject: " + m.Subject + "\n\n" + a.body
	a.composeAttachments = nil
	a.startComposeDoc()
}

func (a *App) archiveSelected() {
	m, folder, uid, index, ok := a.currentTarget()
	if !ok {
		return
	}
	sess := a.sess
	if sess == nil {
		return
	}
	dest := a.folderName("archive")
	if dest == "" {
		a.errMsg = "No Archive folder found."
		a.invalidateAll()
		return
	}
	email, from := a.messageCacheKey(m)
	netFolder := folder
	if a.folder == "FLAGGED" {
		netFolder = from
	}
	if index >= 0 {
		a.msgs = append(a.msgs[:index], a.msgs[index+1:]...)
		if a.sel >= len(a.msgs) {
			a.sel = len(a.msgs) - 1
		}
	} else {
		a.viewingMsg = nil
	}
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	a.showCompose = false
	var next imap.UID
	if a.viewingMsg == nil && a.sel >= 0 && a.sel < len(a.msgs) {
		next = a.msgs[a.sel].UID
	}
	a.invalidateAll()
	go func() {
		a.netMu.Lock()
		newUID, err := sess.Move(netFolder, uid, dest)
		a.netMu.Unlock()
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			a.loadFolder(folder, true, 0)
			return
		}
		if newUID != 0 {
			m.UID = newUID
		}
		a.recordMove(email, from, m, dest)
		a.invalidateAll()
		if next != 0 {
			a.loadBody(next)
		}
	}()
}

func (a *App) moveToJunkSelected() {
	m, folder, uid, index, ok := a.currentTarget()
	if !ok {
		return
	}
	sess := a.sess
	if sess == nil {
		return
	}
	dest := a.folderName("junk")
	if dest == "" {
		a.errMsg = "No Junk folder found."
		a.invalidateAll()
		return
	}
	email, from := a.messageCacheKey(m)
	netFolder := folder
	if a.folder == "FLAGGED" {
		netFolder = from
	}
	if index >= 0 {
		a.msgs = append(a.msgs[:index], a.msgs[index+1:]...)
		if a.sel >= len(a.msgs) {
			a.sel = len(a.msgs) - 1
		}
	} else {
		a.viewingMsg = nil
	}
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	a.showCompose = false
	var next imap.UID
	if a.viewingMsg == nil && a.sel >= 0 && a.sel < len(a.msgs) {
		next = a.msgs[a.sel].UID
	}
	a.invalidateAll()
	go func() {
		a.netMu.Lock()
		newUID, err := sess.Move(netFolder, uid, dest)
		a.netMu.Unlock()
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			a.loadFolder(folder, true, 0)
			return
		}
		if newUID != 0 {
			m.UID = newUID
		}
		a.recordMove(email, from, m, dest)
		a.invalidateAll()
		if next != 0 {
			a.loadBody(next)
		}
	}()
}

func (a *App) moveToInboxSelected() {
	m, folder, uid, index, ok := a.currentTarget()
	if !ok {
		return
	}
	sess := a.sess
	if sess == nil {
		return
	}
	dest := "INBOX"
	email, from := a.messageCacheKey(m)
	netFolder := folder
	if a.folder == "FLAGGED" {
		netFolder = from
	}
	if index >= 0 {
		a.msgs = append(a.msgs[:index], a.msgs[index+1:]...)
		if a.sel >= len(a.msgs) {
			a.sel = len(a.msgs) - 1
		}
	} else {
		a.viewingMsg = nil
	}
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	a.showCompose = false
	var next imap.UID
	if a.viewingMsg == nil && a.sel >= 0 && a.sel < len(a.msgs) {
		next = a.msgs[a.sel].UID
	}
	a.invalidateAll()
	go func() {
		a.netMu.Lock()
		newUID, err := sess.Move(netFolder, uid, dest)
		a.netMu.Unlock()
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			a.loadFolder(folder, true, 0)
			return
		}
		if newUID != 0 {
			m.UID = newUID
		}
		a.recordMove(email, from, m, dest)
		a.invalidateAll()
		if next != 0 {
			a.loadBody(next)
		}
	}()
}

func (a *App) closeCompose() {
	a.showCompose = false
	a.composeDoc = nil
	a.composeAttachments = nil
	a.composeErr = ""
	a.composeQuote = ""
	a.composeCc = ""
}

func (a *App) reply() {
	m, _, _, _, ok := a.currentTarget()
	if !ok {
		return
	}
	subject := m.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	a.showCompose = true
	a.composeTo = m.FromAddr
	a.composeCc = ""
	a.composeSub = subject
	a.composeBody = ""
	a.composeQuote = "\n\nOn " + m.Date.Format("Mon, 2 Jan 2006 at 15:04") + ", " + m.FromName + " wrote:\n" + a.body
	a.composeAttachments = nil
	a.startComposeDoc()
}

func (a *App) setSelectedSeen(seen bool) {
	if a.sel < 0 || a.sel >= len(a.msgs) {
		return
	}
	a.setMessageSeen(a.sel, seen)
}

func (a *App) setMessageSeen(i int, seen bool) {
	if i < 0 || i >= len(a.msgs) {
		return
	}
	m := &a.msgs[i]
	if m.Read == seen {
		return
	}
	m.Read = seen
	msg := *m
	email, folder := a.messageCacheKey(msg)
	if seen {
		a.adjustUnread(a.folder, -1)
	} else {
		a.adjustUnread(a.folder, 1)
	}
	uid := msg.UID
	go func() {
		a.netMu.Lock()
		_ = a.sess.SetSeen(folder, uid, seen)
		a.netMu.Unlock()
		a.cacheUpdateMessageAt(email, folder, msg, func(c *Message) { c.Read = seen })
		a.saveDiskCache()
		a.invalidateAll()
	}()
}

func (a *App) toggleFlag(i int) {
	if i < 0 || i >= len(a.msgs) {
		return
	}
	m := &a.msgs[i]
	m.Flagged = !m.Flagged
	flagged := m.Flagged
	msg := *m
	email, folder := a.messageCacheKey(msg)
	uid := msg.UID
	// In the Flagged view, remove the message once unflagged.
	if a.folder == "FLAGGED" && !flagged {
		a.msgs = append(a.msgs[:i], a.msgs[i+1:]...)
		if a.sel == i {
			if i < len(a.msgs) {
				a.sel = i
			} else {
				a.sel = len(a.msgs) - 1
			}
			a.body = ""
			a.bodyUID = 0
			a.bodyHTML = ""
			a.viewAsText = false
		} else if a.sel > i {
			a.sel--
		}
	}
	go func() {
		a.netMu.Lock()
		_ = a.sess.SetFlagged(folder, uid, flagged)
		a.netMu.Unlock()
		a.cacheUpdateMessageAt(email, folder, msg, func(c *Message) { c.Flagged = flagged })
		if flagged {
			msg.Folder = folder
			a.cacheAddToFlagged(email, msg)
		} else {
			a.cacheRemoveFromFlagged(email, msg)
		}
		a.saveDiskCache()
		a.invalidateAll()
	}()
}

func (a *App) deleteSelected() {
	m, folder, uid, index, ok := a.currentTarget()
	if !ok {
		return
	}
	sess := a.sess
	if sess == nil {
		return
	}
	email, from := a.messageCacheKey(m)
	netFolder := folder
	if a.folder == "FLAGGED" {
		netFolder = from
	}
	trash := a.folderName("trash")
	if trash == "" || strings.EqualFold(from, trash) {
		trash = ""
	}
	wasUnread := !m.Read
	if index >= 0 {
		a.msgs = append(a.msgs[:index], a.msgs[index+1:]...)
		if a.sel >= len(a.msgs) {
			a.sel = len(a.msgs) - 1
		}
	} else {
		a.viewingMsg = nil
	}
	a.body = ""
	a.bodyUID = 0
	a.bodyHTML = ""
	a.viewAsText = false
	a.showCompose = false
	if wasUnread {
		a.adjustUnread(folder, -1)
	}
	var next imap.UID
	if a.viewingMsg == nil && a.sel >= 0 && a.sel < len(a.msgs) {
		next = a.msgs[a.sel].UID
	}
	a.invalidateAll()

	go func() {
		a.netMu.Lock()
		newUID, err := sess.Delete(netFolder, uid)
		a.netMu.Unlock()
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			a.loadFolder(folder, true, 0)
			return
		}
		if newUID != 0 {
			m.UID = newUID
		}
		a.recordMove(email, from, m, trash)
		a.invalidateAll()
		if next != 0 {
			a.loadBody(next)
		}
	}()
}

func (a *App) send() {
	to := strings.TrimSpace(a.composeTo)
	cc := strings.TrimSpace(a.composeCc)
	sub := strings.TrimSpace(a.composeSub)
	plain, htmlBody, inlineData, inlineTypes := a.composeMessageParts()
	acct := a.acct
	sess := a.sess
	atts := append([]composeAttachment(nil), a.composeAttachments...)
	recipients := to
	if cc != "" {
		if recipients != "" {
			recipients += ", "
		}
		recipients += cc
	}
	sentTo := to
	if sentTo == "" {
		sentTo = cc
	}
	var sent string
	if sess != nil {
		sent = a.folderName("sent")
	}
	a.sending = true
	a.composeErr = ""
	a.invalidateAll()

	go func() {
		outgoing, err := readOutgoingAttachments(atts)
		if err != nil {
			a.mu.Lock()
			a.sending = false
			a.composeErr = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		var raw []byte
		if htmlBody != "" {
			raw = BuildRichMessage(acct, to, cc, sub, plain, htmlBody, inlineData, inlineTypes, outgoing)
		} else {
			raw = BuildMessage(acct, to, cc, sub, plain, outgoing)
		}
		err = Send(acct, recipients, raw)
		a.mu.Lock()
		a.sending = false
		if err != nil {
			a.composeErr = err.Error()
		} else {
			a.showCompose = false
			a.composeTo, a.composeCc, a.composeSub, a.composeBody, a.composeQuote = "", "", "", "", ""
			a.composeDoc = nil
			a.composeAttachments = nil
			a.note = "Message sent to " + sentTo
			if sess != nil && sent != "" {
				a.netMu.Lock()
				_ = sess.Append(sent, raw)
				a.netMu.Unlock()
			}
		}
		a.mu.Unlock()
		a.invalidateAll()
	}()
}

// windowControls draws the minimize, maximize and close buttons. The window is
// frameless, so these are ours to draw and they stay legible in every theme.
func (a *App) windowControls(c *ui.Context, win *mygo.Window) {
	maxIcon := "maximize"
	maxLabel := "Maximize"
	if win != nil && win.IsMaximized() {
		maxIcon = "restore"
		maxLabel = "Restore"
	}
	ui.Row(c).Gap(2).Shrink(0).Children(func() {
		min := a.windowControl(c, "window:minimize", "minimize", "Minimize", false)
		if min.Clicked() && win != nil {
			win.Minimize()
		}
		max := a.windowControl(c, "window:"+maxIcon, maxIcon, maxLabel, false)
		if max.Clicked() && win != nil {
			win.ToggleMaximize()
		}
		close := a.windowControl(c, "window:close", "x", "Close", true)
		if close.Clicked() && win != nil {
			a.closePopovers()
			win.Close()
		}
	})
}

func (a *App) windowControl(c *ui.Context, key, name, label string, danger bool) ui.Element {
	p := a.palette
	b := ui.ButtonBase(c).Key(key).Size(36, 30).Radius(7).Center().Label(label).Tooltip(label).FocusRing(false)
	iconColor := p.Muted
	if b.Hovered() {
		iconColor = p.Text
		if danger {
			b.Background(p.Bad)
			iconColor = contrastText(p.Bad)
		} else {
			b.Background(p.Hover)
		}
	}
	if b.Pressed() {
		if danger {
			b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.12))
			iconColor = contrastText(p.Bad)
		} else {
			b.Background(p.Border)
		}
	}
	b.Children(func() {
		ui.Icon(c, icon(name, 14)).Size(14, 14).TextColor(iconColor)
	})
	return b
}

func (a *App) logout() {
	a.mu.Lock()
	a.keepAliveGen = 0
	if a.keepAliveStop != nil {
		close(a.keepAliveStop)
		a.keepAliveStop = nil
	}
	a.mu.Unlock()
	sess := a.sess
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts = append(a.accounts[:a.active], a.accounts[a.active+1:]...)
	}
	if a.active >= len(a.accounts) && len(a.accounts) > 0 {
		a.active = len(a.accounts) - 1
	}
	a.sess = nil
	a.connected = false
	a.connecting = false
	a.switching = false
	a.msgs = nil
	a.folders = nil
	a.sel = -1
	a.viewingMsg = nil
	a.resetSearch()
	a.body = ""
	a.body = ""
	a.bodyUID = 0
	a.showCompose = false
	a.errMsg = ""
	a.note = ""
	a.form = Account{IMAPPort: "993", SMTPPort: "587"}
	if len(a.accounts) > 0 && a.active >= 0 && a.active < len(a.accounts) {
		a.form = a.accounts[a.active]
	}
	a.saveConfig()
	a.invalidateAll()
	if sess != nil {
		go func() {
			a.netMu.Lock()
			sess.Close()
			a.netMu.Unlock()
		}()
	}
}

func (a *App) folderLabel() string {
	if a.folder == "SHARED" {
		return "Shared Mailbox"
	}
	for _, f := range a.folders {
		if f.Name == a.folder {
			return f.Label
		}
	}
	return a.folder
}

func (a *App) folderName(kind string) string {
	for _, f := range a.folders {
		if f.Kind == kind {
			return f.Name
		}
	}
	return ""
}

func (a *App) adjustUnread(folder string, delta int) {
	for i := range a.folders {
		if a.folders[i].Name == folder {
			a.folders[i].Unread += delta
			if a.folders[i].Unread < 0 {
				a.folders[i].Unread = 0
			}
			return
		}
	}
}

func (a *App) displayName() string {
	if a.acct.Name != "" {
		return a.acct.Name
	}
	if i := strings.Index(a.acct.Email, "@"); i > 0 {
		return a.acct.Email[:i]
	}
	return a.acct.Email
}

func firstUnread(msgs []Message) int {
	for i, m := range msgs {
		if !m.Read {
			return i
		}
	}
	if len(msgs) > 0 {
		return 0
	}
	return -1
}

func shortDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("2 Jan")
	}
	return t.Format("02/01/06")
}

// ------------------------------------------------------------- actions ---

// applyAccountAppearance loads an account's saved theme into the model. It
// runs both when the account is switched and again once its connection
// succeeds, so the chrome follows the account immediately and stays correct
// after the connection is established.
//
// This only touches the model: window work such as SetVibrancy has to happen
// after the app mutex is released. Calling it under the mutex from a
// background goroutine deadlocks, because setting the material waits for the
// main thread while the main thread's frame is waiting for the mutex.
func (a *App) applyAccountAppearance(acct Account) {
	if acct.Theme.Theme != "" {
		a.themeSettings = acct.Theme
	}
	a.palette = resolvePalette(a.themeSettings, a.systemDark, a.customThemes)
	activePalette = a.palette
	a.dark = a.palette.Dark
}

func (a *App) switchAccount(i int) {
	if i < 0 || i >= len(a.accounts) {
		return
	}
	a.mu.Lock()
	acct := a.accounts[i]
	acct.Password = strings.TrimSpace(acct.Password)
	a.switchFrom = a.active
	a.active = i
	a.form = acct
	a.switching = true
	a.connecting = true
	a.keepAliveGen = 0
	if a.keepAliveStop != nil {
		close(a.keepAliveStop)
		a.keepAliveStop = nil
	}
	a.applyAccountAppearance(acct)
	a.mu.Unlock()
	a.requestWindowMaterials()
	a.invalidateAll()
	go a.doConnect(acct)
}

func (a *App) startAddAccount() {
	a.mu.Lock()
	a.keepAliveGen = 0
	if a.keepAliveStop != nil {
		close(a.keepAliveStop)
		a.keepAliveStop = nil
	}
	a.mu.Unlock()
	sess := a.sess
	a.sess = nil
	a.connected = false
	a.connecting = false
	a.switching = false
	a.msgs = nil
	a.folders = nil
	a.sel = -1
	a.viewingMsg = nil
	a.resetSearch()
	a.body = ""
	a.body = ""
	a.bodyUID = 0
	a.showCompose = false
	a.errMsg = ""
	a.note = ""
	a.form = Account{IMAPPort: "993", SMTPPort: "587"}
	if sess != nil {
		go func() {
			a.netMu.Lock()
			sess.Close()
			a.netMu.Unlock()
		}()
	}
}

func (a *App) selectFolder(name string) {
	a.loadFolder(name, false, 0)
}

func (a *App) loadFolder(name string, force bool, offset int) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("loadFolder", r)
		}
	}()
	// Remember the folder we're leaving, and reuse a folder we've already
	// loaded instead of refetching it, so scrolled-in pages stay put.
	a.mu.Lock()
	conn := a.connID
	if a.cache == nil {
		a.cache = map[string]*folderCache{}
	}
	if a.folder != "" && a.folder != name {
		a.cache[a.folder] = &folderCache{
			msgs: a.msgs, loadedLimit: a.loadedLimit, hasMore: a.hasMore,
			sel: a.sel, scrollY: a.scrollState.Y,
		}
	}
	if name != a.folder && !force && offset == 0 {
		if entry := a.cache[name]; entry != nil {
			a.folder = name
			a.msgs = entry.msgs
			a.loadedLimit = entry.loadedLimit
			a.hasMore = entry.hasMore
			a.sel = entry.sel
			a.viewingMsg = nil
			a.resetSearch()
			a.body = ""
			a.bodyUID = 0
			a.bodyHTML = ""
			a.viewAsText = false
			a.errMsg = ""
			a.loadingMsgs = false
			a.scrollState.Y = entry.scrollY
			var selUID imap.UID
			if a.sel >= 0 && a.sel < len(a.msgs) {
				selUID = a.msgs[a.sel].UID
			}
			a.mu.Unlock()
			a.invalidateAll()
			if selUID != 0 {
				go a.loadBody(selUID)
			}
			return
		}
	}
	a.mu.Unlock()
	switching := (name != a.folder)
	// Virtual "Flagged" folder: show flagged messages from ALL mailboxes.
	if name == "FLAGGED" {
		a.mu.Lock()
		if !force && a.folder == "FLAGGED" {
			a.mu.Unlock()
			return
		}
		a.folder = "FLAGGED"
		a.msgs = nil
		a.sel = -1
		a.viewingMsg = nil
		a.filterQuery = ""
		a.body = ""
		a.bodyUID = 0
		a.bodyHTML = ""
		a.viewAsText = false
		a.errMsg = ""
		a.loadingMsgs = true
		a.mu.Unlock()
		a.invalidateAll()
		if a.sess == nil {
			a.mu.Lock()
			a.loadingMsgs = false
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		msgs, err := a.sess.SearchFlagged("")
		a.netMu.Lock()
		var flagged []Message
		for _, m := range msgs {
			flagged = append(flagged, m)
		}
		a.netMu.Unlock()
		a.mu.Lock()
		a.loadingMsgs = false
		if a.folder != "FLAGGED" {
			a.mu.Unlock()
			return
		}
		if err != nil {
			a.errMsg = err.Error()
		} else {
			// Deduplicate by UID across mailboxes
			seen := map[imap.UID]bool{}
			for _, m := range flagged {
				if !seen[m.UID] {
					seen[m.UID] = true
					a.msgs = append(a.msgs, m)
				}
			}
			a.sel = firstUnread(a.msgs)
			a.hasMore = false
			a.cache["FLAGGED"] = &folderCache{
				msgs: a.msgs, loadedLimit: len(a.msgs), hasMore: false,
				sel: a.sel, scrollY: a.scrollState.Y,
			}
		}
		a.mu.Unlock()
		a.invalidateAll()
		return
	}
	// Virtual "Shared" mailbox: aggregate INBOX messages from every account.
	if name == "SHARED" {
		a.mu.Lock()
		if !force && a.folder == "SHARED" {
			a.mu.Unlock()
			return
		}
		a.folder = "SHARED"
		a.msgs = nil
		a.sel = -1
		a.viewingMsg = nil
		a.filterQuery = ""
		a.body = ""
		a.bodyUID = 0
		a.bodyHTML = ""
		a.viewAsText = false
		a.errMsg = ""
		a.loadingMsgs = true
		a.hasMore = false
		a.loadedLimit = 0
		a.mu.Unlock()
		a.invalidateAll()

		if len(a.accounts) == 0 {
			a.mu.Lock()
			a.loadingMsgs = false
			a.mu.Unlock()
			a.invalidateAll()
			return
		}

		// Snapshot accounts under lock.
		a.mu.Lock()
		accounts := append([]Account(nil), a.accounts...)
		a.mu.Unlock()

		// First, take everything we already have cached for each account.
		seen := map[string]bool{}
		var collected []Message
		a.mu.Lock()
		for _, acc := range accounts {
			dc := a.diskCache.Accounts[acc.Email]
			if dc == nil {
				continue
			}
			// Copy before rewriting Folder: the cached slice belongs to the
			// disk cache and must keep its physical folder names.
			cached := append([]Message(nil), dc["INBOX"]...)
			for i := range cached {
				cached[i].Folder = acc.Email
			}
			for _, m := range cached {
				key := acc.Email + "|" + fmt.Sprint(m.UID)
				if seen[key] {
					continue
				}
				seen[key] = true
				collected = append(collected, m)
			}
		}
		a.mu.Unlock()
		// Then fetch the latest page from each account and merge new mail.
		for _, acc := range accounts {
			sess, _, err := Open(acc)
			if err != nil {
				continue
			}
			msgs, err := sess.Messages("INBOX", 100, 0)
			sess.Close()
			if err != nil {
				continue
			}
			for i := range msgs {
				msgs[i].Folder = acc.Email
			}
			for _, m := range msgs {
				key := acc.Email + "|" + fmt.Sprint(m.UID)
				if seen[key] {
					// Update read/flagged state for cached entries.
					for i := range collected {
						if collected[i].Folder == acc.Email && collected[i].UID == m.UID {
							collected[i].Read = m.Read
							collected[i].Flagged = m.Flagged
						}
					}
					continue
				}
				seen[key] = true
				collected = append(collected, m)
			}
		}
		// Newest first.
		// Simple sort by date descending.
		// (stable not required)
		// Use slices.SortFunc if available; fallback manual.
		// We'll use a simple insertion sort to avoid imports.
		for i := 1; i < len(collected); i++ {
			j := i
			for j > 0 && collected[j-1].Date.Before(collected[j].Date) {
				collected[j-1], collected[j] = collected[j], collected[j-1]
				j--
			}
		}

		a.mu.Lock()
		if a.folder != "SHARED" {
			a.mu.Unlock()
			return
		}
		a.msgs = collected
		a.loadedLimit = len(collected)
		a.hasMore = false
		a.loadingMsgs = false
		a.sel = firstUnread(collected)
		a.cache["SHARED"] = &folderCache{
			msgs: a.msgs, loadedLimit: a.loadedLimit, hasMore: false,
			sel: a.sel, scrollY: a.scrollState.Y,
		}
		a.mu.Unlock()
		a.invalidateAll()
		return
	}

	a.mu.Lock()
	if name == "" || (!force && name == a.folder) {
		a.mu.Unlock()
		return
	}
	sess := a.sess
	loadMore := !switching && force && offset > 0
	if switching {
		a.loadedLimit = 100
		a.hasMore = true
		a.loadingMore = false
	}
	if !loadMore {
		a.folder = name
		a.msgs = nil
		a.sel = -1
		a.viewingMsg = nil
		a.filterQuery = ""
		a.body = ""
		a.bodyUID = 0
		a.bodyHTML = ""
		a.viewAsText = false
		a.errMsg = ""
		a.loadingMsgs = true
	}
	a.mu.Unlock()
	a.invalidateAll()

	if sess == nil {
		a.mu.Lock()
		a.loadingMsgs = false
		a.mu.Unlock()
		a.invalidateAll()
		return
	}
	fetchOffset := 0
	if loadMore {
		fetchOffset = offset
	}
	a.netMu.Lock()
	msgs, err := sess.Messages(name, 100, fetchOffset)
	a.netMu.Unlock()

	a.mu.Lock()
	if a.connID != conn {
		a.mu.Unlock()
		return
	}
	if !loadMore && a.folder != name {
		a.mu.Unlock()
		return
	}
	if loadMore && a.folder != name {
		a.mu.Unlock()
		return
	}
	if err != nil {
		a.errMsg = err.Error()
	} else {
		a.errMsg = ""
		for i := range msgs {
			msgs[i].Folder = name
		}
		if loadMore {
			existing := map[imap.UID]bool{}
			for _, m := range a.msgs {
				existing[m.UID] = true
			}
			for _, m := range msgs {
				if !existing[m.UID] {
					a.msgs = append(a.msgs, m)
				}
			}
			a.loadedLimit = len(a.msgs)
			a.hasMore = len(msgs) >= 100
		} else {
			a.msgs = msgs
			a.loadedLimit = len(msgs)
			if a.loadedLimit < 100 {
				a.loadedLimit = 100
			}
			a.hasMore = len(msgs) >= 100
			a.sel = firstUnread(msgs)
		}
		a.cache[name] = &folderCache{
			msgs: a.msgs, loadedLimit: a.loadedLimit, hasMore: a.hasMore,
			sel: a.sel, scrollY: a.scrollState.Y,
		}
	}
	a.loadingMsgs = false
	a.mu.Unlock()
	a.invalidateAll()
}

// messageHTMLDoc wraps an email's HTML for the reader webview. Many HTML mails
// hard-code a dark text colour (on <body>, <font> or individual cells). On the
// dark reader canvas that text is invisible until selected. A stylesheet rule
// with !important wins over those inline colours, so the text stays readable;
// links keep their own accent. Newsletters that explicitly declare a white
// canvas get a light wrapper instead, where dark text is the right choice.
func (a *App) messageHTMLDoc(htmlBody string) string {
	body := resolveSignatureCIDs(htmlBody, a.acct)
	fg, bg, link := "#e7e7ea", "#15161c", "#7db4ff"
	if htmlHasLightBackground(body) {
		fg, bg, link = "#1a1a1a", "#ffffff", "#1558d6"
	}
	css := fmt.Sprintf(`body{font-family:"Segoe UI",Arial,sans-serif;font-size:14px;margin:12px;word-wrap:break-word;background:%s;color:%s;}body,body *{color:%s!important;}a,a *{color:%s!important;}img{max-width:100%%;height:auto;}`, bg, fg, fg, link)
	return "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><style>" + css + "</style></head><body>" + body + "</body></html>"
}

// htmlHasLightBackground spots the common way newsletters declare a white
// canvas, so their own dark text is kept rather than inverted.
func htmlHasLightBackground(htmlBody string) bool {
	s := strings.ToLower(htmlBody)
	for _, needle := range []string{
		`bgcolor="#ffffff"`, `bgcolor='#ffffff'`, `bgcolor="#fff"`, `bgcolor="white"`,
		`background:#fff`, `background: #fff`, `background:#ffffff`, `background: #ffffff`,
		`background-color:#fff`, `background-color: #fff`, `background-color:#ffffff`,
		`background-color: #ffffff`, `background:white`, `background-color:white`,
	} {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func (a *App) loadBody(uid imap.UID) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("loadBody", r)
		}
	}()
	a.mu.Lock()
	if !a.connected || a.sess == nil || uid == 0 {
		a.mu.Unlock()
		return
	}
	folder := a.folder
	if folder == "FLAGGED" {
		for i := range a.msgs {
			if a.msgs[i].UID == uid && a.msgs[i].Folder != "" {
				folder = a.msgs[i].Folder
				break
			}
		}
	}
	if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
		folder = "INBOX"
	}
	sess := a.sess
	wasUnread := false
	for _, m := range a.msgs {
		if m.UID == uid && !m.Read {
			wasUnread = true
		}
	}
	a.bodyLoading = true
	a.bodyUID = uid
	a.body = ""
	a.bodyHTML = ""
	a.viewAsText = false
	a.mu.Unlock()
	a.invalidateAll()

	a.netMu.Lock()
	body, htmlBody, attachments, err := sess.Body(folder, uid)
	if err == nil {
		err = sess.SetSeen(folder, uid, true)
	}
	a.netMu.Unlock()

	a.mu.Lock()
	if a.bodyUID != uid {
		a.mu.Unlock()
		return
	}
	a.bodyLoading = false
	if err != nil {
		a.body = "Could not load this message.\n\n" + err.Error()
		a.bodyHTML = ""
	} else {
		if strings.TrimSpace(body) == "" && strings.TrimSpace(htmlBody) == "" {
			body = "(This message has no text content.)"
		}
		a.body = body
		a.bodyHTML = htmlBody
		for i := range a.msgs {
			if a.msgs[i].UID == uid {
				a.msgs[i].Read = true
				a.msgs[i].Body = body
				a.msgs[i].Attachments = attachments
			}
		}
		if wasUnread {
			a.adjustUnread(folder, -1)
		}
		if htmlBody != "" && a.readerView != nil {
			wrapped := a.messageHTMLDoc(htmlBody)
			a.readerView.Page().LoadHTML(wrapped, "")
		}
	}
	a.mu.Unlock()
	if err == nil {
		a.cacheUpdateMessageAt(a.acct.Email, folder, Message{UID: uid, Folder: folder}, func(c *Message) {
			c.Read = true
			c.Body = body
			c.Attachments = attachments
		})
	}
	a.saveDiskCache()
	a.invalidateAll()
}

// loadBodyFrom fetches and shows one message from any folder, without touching
// the loaded message list (used when opening a search result).
func (a *App) loadBodyFrom(folder string, uid imap.UID) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("loadBodyFrom", r)
		}
	}()
	a.mu.Lock()
	if !a.connected || a.sess == nil || folder == "" || uid == 0 {
		a.mu.Unlock()
		return
	}
	sess := a.sess
	if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
		folder = "INBOX"
	}
	a.bodyLoading = true
	a.bodyUID = uid
	a.body = ""
	a.bodyHTML = ""
	a.viewAsText = false
	a.mu.Unlock()
	a.invalidateAll()

	a.netMu.Lock()
	body, htmlBody, attachments, err := sess.Body(folder, uid)
	if err == nil {
		err = sess.SetSeen(folder, uid, true)
	}
	a.netMu.Unlock()

	a.mu.Lock()
	if a.bodyUID != uid {
		a.mu.Unlock()
		return
	}
	a.bodyLoading = false
	if err != nil {
		a.body = "Could not load this message.\n\n" + err.Error()
		a.bodyHTML = ""
	} else {
		if strings.TrimSpace(body) == "" && strings.TrimSpace(htmlBody) == "" {
			body = "(This message has no text content.)"
		}
		a.body = body
		a.bodyHTML = htmlBody
		if a.viewingMsg != nil && a.viewingUID == uid {
			a.viewingMsg.Read = true
			a.viewingMsg.Body = body
			a.viewingMsg.Attachments = attachments
		}
		if htmlBody != "" && a.readerView != nil {
			wrapped := a.messageHTMLDoc(htmlBody)
			a.readerView.Page().LoadHTML(wrapped, "")
		}
	}
	a.mu.Unlock()
	if err == nil {
		a.cacheUpdateMessageAt(a.acct.Email, folder, Message{UID: uid, Folder: folder}, func(c *Message) {
			c.Read = true
			c.Body = body
			c.Attachments = attachments
		})
		a.saveDiskCache()
	}
	a.invalidateAll()
}

func (a *App) loadMore() {
	if a.loadedLimit <= 0 {
		a.loadedLimit = 100
	}
	offset := a.loadedLimit
	a.loadedLimit += 100
	go a.loadFolder(a.folder, true, offset)
}

var accountColors = []ui.Color{
	ui.RGB(92, 168, 255),
	ui.RGB(255, 107, 107),
	ui.RGB(74, 222, 128),
	ui.RGB(251, 191, 36),
	ui.RGB(192, 132, 252),
	ui.RGB(244, 114, 182),
}

func (a *App) accountColor(email string) ui.Color {
	for i, acc := range a.accounts {
		if acc.Email == email {
			return accountColors[i%len(accountColors)]
		}
	}
	// Fallback: hash the email.
	h := 0
	for _, r := range email {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return accountColors[h%len(accountColors)]
}

func (a *App) accountByEmail(email string) (Account, bool) {
	for _, acc := range a.accounts {
		if acc.Email == email {
			return acc, true
		}
	}
	return Account{}, false
}

func (a *App) loadSharedBody(m Message) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("loadSharedBody", r)
		}
	}()
	a.mu.Lock()
	a.bodyLoading = true
	a.bodyUID = m.UID
	a.body = ""
	a.bodyHTML = ""
	a.viewAsText = false
	a.mu.Unlock()
	a.invalidateAll()

	acct, ok := a.accountByEmail(m.Folder)
	if !ok {
		a.mu.Lock()
		if a.bodyUID == m.UID {
			a.bodyLoading = false
			a.body = "Could not load this message.\n\nUnknown account."
		}
		a.mu.Unlock()
		a.invalidateAll()
		return
	}
	sess, _, err := Open(acct)
	if err != nil {
		a.mu.Lock()
		if a.bodyUID == m.UID {
			a.bodyLoading = false
			a.body = "Could not load this message.\n\n" + err.Error()
		}
		a.mu.Unlock()
		a.invalidateAll()
		return
	}
	body, htmlBody, attachments, err := sess.Body("INBOX", m.UID)
	if err == nil {
		_ = sess.SetSeen("INBOX", m.UID, true)
	}
	sess.Close()

	a.mu.Lock()
	if a.bodyUID != m.UID {
		a.mu.Unlock()
		return
	}
	a.bodyLoading = false
	if err != nil {
		a.body = "Could not load this message.\n\n" + err.Error()
		a.bodyHTML = ""
	} else {
		if strings.TrimSpace(body) == "" && strings.TrimSpace(htmlBody) == "" {
			body = "(This message has no text content.)"
		}
		a.body = body
		a.bodyHTML = htmlBody
		for i := range a.msgs {
			if a.msgs[i].UID == m.UID && a.msgs[i].Folder == m.Folder {
				a.msgs[i].Read = true
				a.msgs[i].Body = body
				a.msgs[i].Attachments = attachments
			}
		}
		if htmlBody != "" && a.readerView != nil {
			wrapped := a.messageHTMLDoc(htmlBody)
			a.readerView.Page().LoadHTML(wrapped, "")
		}
	}
	a.mu.Unlock()
	if err == nil {
		a.cacheUpdateMessageAt(m.Folder, "INBOX", m, func(c *Message) {
			c.Read = true
			c.Body = body
			c.Attachments = attachments
		})
	}
	a.saveDiskCache()
	a.invalidateAll()
}

// mailPollInterval is how often the open mailbox is checked for new mail.
// Polling only appends what arrived; it never reloads what is already shown.
const mailPollInterval = 60 * time.Second

func (a *App) startKeepAlive() {
	defer func() {
		if r := recover(); r != nil {
			logPanic("startKeepAlive", r)
		}
	}()
	a.mu.Lock()
	if a.keepAliveStop != nil {
		close(a.keepAliveStop)
	}
	a.keepAliveGen++
	gen := a.keepAliveGen
	stop := make(chan struct{})
	a.keepAliveStop = stop
	a.mu.Unlock()

	go func() {
		ticker := time.NewTicker(90 * time.Second)
		defer ticker.Stop()
		poll := time.NewTicker(mailPollInterval)
		defer poll.Stop()
		for {
			select {
			case <-stop:
				return
			case <-poll.C:
				a.pollNewMail(gen)
			case <-ticker.C:
				a.mu.Lock()
				if a.keepAliveGen != gen || !a.connected || a.sess == nil {
					a.mu.Unlock()
					return
				}
				sess := a.sess
				a.mu.Unlock()

				a.netMu.Lock()
				err := sess.Noop()
				a.netMu.Unlock()
				if err == nil {
					continue
				}
				a.mu.Lock()
				if a.keepAliveGen != gen {
					a.mu.Unlock()
					return
				}
				a.mu.Unlock()
				a.reconnect()
				return
			}
		}
	}()
}

func (a *App) reopenSession() bool {
	defer func() {
		if r := recover(); r != nil {
			logPanic("reopenSession", r)
		}
	}()
	a.mu.Lock()
	acct := a.acct
	a.mu.Unlock()
	if acct.Email == "" {
		return false
	}

	a.netMu.Lock()
	sess, folders, err := Open(acct)
	a.netMu.Unlock()
	if err != nil {
		return false
	}

	a.mu.Lock()
	old := a.sess
	a.sess = sess
	if folders != nil {
		hasFlagged := false
		for _, f := range folders {
			if f.Kind == "flagged" {
				hasFlagged = true
				break
			}
		}
		if !hasFlagged {
			folders = append(folders, Folder{Name: "FLAGGED", Label: "Flagged", Kind: "flagged"})
		}
		if len(a.acct.FolderOrder) > 0 {
			byName := map[string]Folder{}
			for _, f := range folders {
				byName[f.Name] = f
			}
			matched := 0
			for _, name := range a.acct.FolderOrder {
				if _, ok := byName[name]; ok {
					matched++
				}
			}
			if matched >= len(folders)/2 || matched >= 3 {
				ordered := make([]Folder, 0, len(folders))
				seen := map[string]bool{}
				for _, name := range a.acct.FolderOrder {
					if f, ok := byName[name]; ok && !seen[name] {
						ordered = append(ordered, f)
						seen[name] = true
					}
				}
				for _, f := range folders {
					if !seen[f.Name] {
						ordered = append(ordered, f)
					}
				}
				folders = ordered
			} else {
				slices.SortStableFunc(folders, func(a, b Folder) int {
					if ra, rb := folderRank(a.Kind), folderRank(b.Kind); ra != rb {
						return ra - rb
					}
					return strings.Compare(a.Label, b.Label)
				})
			}
		} else {
			slices.SortStableFunc(folders, func(a, b Folder) int {
				if ra, rb := folderRank(a.Kind), folderRank(b.Kind); ra != rb {
					return ra - rb
				}
				return strings.Compare(a.Label, b.Label)
			})
		}
		a.folders = folders
	}
	a.connected = true
	a.connecting = false
	a.switching = false
	a.connID++
	a.bgFetching = true
	a.fetchGen++
	gen := a.fetchGen
	a.mu.Unlock()

	if old != nil && old != sess {
		go func() {
			a.netMu.Lock()
			old.Close()
			a.netMu.Unlock()
		}()
	}
	a.startKeepAlive()
	go a.fetchAllMailboxes(gen)
	return true
}

func (a *App) reconnect() {
	defer func() {
		if r := recover(); r != nil {
			logPanic("reconnect", r)
		}
	}()
	for attempt := 0; attempt < 5; attempt++ {
		a.mu.Lock()
		if a.switching || a.connecting || a.reconnecting {
			a.reconnecting = false
			a.mu.Unlock()
			return
		}
		a.reconnecting = true
		a.mu.Unlock()

		ok := a.reopenSession()

		a.mu.Lock()
		a.reconnecting = false
		a.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}
}

// pollNewMail checks the open mailbox for mail that arrived since the last
// poll and adds it to the top of the list. Unlike refresh, it never replaces
// the messages already loaded, so the selection, scroll and fetched pages
// stay where the user left them. New mail is appended only.
func (a *App) pollNewMail(gen int) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("pollNewMail", r)
		}
	}()
	a.mu.Lock()
	if a.keepAliveGen != gen || !a.connected || a.sess == nil || a.switching ||
		a.connecting || a.reconnecting || a.bgFetching || a.loadingMsgs ||
		a.loadingMore || a.filterQuery != "" {
		a.mu.Unlock()
		return
	}
	folder := a.folder
	sess := a.sess
	a.mu.Unlock()
	if folder == "" || folder == "FLAGGED" || folder == "SHARED" {
		return
	}

	a.netMu.Lock()
	fresh, err := sess.Messages(folder, 100, 0)
	a.netMu.Unlock()
	if err != nil {
		return
	}
	for i := range fresh {
		fresh[i].Folder = folder
	}

	a.mu.Lock()
	if a.keepAliveGen != gen || !a.connected || a.folder != folder {
		a.mu.Unlock()
		return
	}
	m := mergeNewMessages(a.msgs, fresh, a.sel)
	if !m.changed {
		a.mu.Unlock()
		return
	}
	a.msgs = m.msgs
	a.sel = m.sel
	a.loadedLimit += m.added
	a.cache[folder] = &folderCache{
		msgs: a.msgs, loadedLimit: a.loadedLimit, hasMore: a.hasMore,
		sel: a.sel, scrollY: a.scrollState.Y,
	}
	if m.unreadDelta != 0 {
		a.adjustUnread(folder, m.unreadDelta)
	}
	a.mu.Unlock()
	a.saveDiskCache()
	a.invalidateAll()
}

// mergeResult is what one poll changed in a mailbox.
type mergeResult struct {
	msgs        []Message
	sel         int
	added       int
	unreadDelta int
	changed     bool
}

// mergeNewMessages merges freshly fetched, newest-first envelopes into the
// list already on screen. Existing messages stay in place and keep any body
// they have; only their read and flagged state follows the server. Messages
// that are new go on top, and the selection follows its message by UID.
func mergeNewMessages(cur, fresh []Message, sel int) mergeResult {
	res := mergeResult{msgs: cur, sel: sel}
	var selUID imap.UID
	if sel >= 0 && sel < len(cur) {
		selUID = cur[sel].UID
	}
	index := make(map[imap.UID]int, len(cur))
	for i := range cur {
		index[cur[i].UID] = i
	}
	var extra []Message
	for _, m := range fresh {
		if i, ok := index[m.UID]; ok && i >= 0 {
			if cur[i].Read != m.Read {
				if m.Read {
					res.unreadDelta--
				} else {
					res.unreadDelta++
				}
				cur[i].Read = m.Read
				res.changed = true
			}
			if cur[i].Flagged != m.Flagged {
				cur[i].Flagged = m.Flagged
				res.changed = true
			}
			continue
		}
		index[m.UID] = -1
		if !m.Read {
			res.unreadDelta++
		}
		res.changed = true
		extra = append(extra, m)
	}
	if len(extra) == 0 {
		return res
	}
	res.added = len(extra)
	res.msgs = make([]Message, 0, len(cur)+len(extra))
	res.msgs = append(res.msgs, extra...)
	res.msgs = append(res.msgs, cur...)
	if selUID != 0 {
		for i := range res.msgs {
			if res.msgs[i].UID == selUID {
				res.sel = i
				break
			}
		}
	}
	return res
}

func (a *App) refresh() {
	defer func() {
		if r := recover(); r != nil {
			logPanic("refresh", r)
		}
	}()
	name := a.folder
	go func() {
		a.loadFolder(name, true, 0)
		a.mu.Lock()
		if a.connected {
			a.bgFetching = true
			a.fetchGen++
			gen := a.fetchGen
			a.mu.Unlock()
			a.fetchAllMailboxes(gen)
		} else {
			a.mu.Unlock()
		}
	}()
}

// fetchAllPages loads the rest of the current folder's messages in the
// background, one page at a time, appending each page as it arrives so the
// mailbox fills up without blocking the UI on the first load.
func (a *App) fetchAllMailboxes(gen int) {
	defer func() {
		if r := recover(); r != nil {
			logPanic("fetchAllMailboxes", r)
		}
	}()
	for {
		a.mu.Lock()
		if a.fetchGen != gen || !a.connected || a.switching || a.connecting || a.sess == nil {
			a.bgFetching = false
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		sess := a.sess
		folders := append([]Folder(nil), a.folders...)
		a.mu.Unlock()

		madeProgress := false
		for _, f := range folders {
			if f.Name == "FLAGGED" || f.Name == "SHARED" {
				continue
			}

			a.mu.Lock()
			var loaded int
			var hasMore bool
			if f.Name == a.folder {
				loaded = a.loadedLimit
				hasMore = a.hasMore
			} else if ce, ok := a.cache[f.Name]; ok && ce != nil {
				loaded = ce.loadedLimit
				hasMore = ce.hasMore
			} else {
				loaded = 0
				hasMore = true
			}
			a.mu.Unlock()

			if !hasMore {
				continue
			}

			offset := loaded
			if offset < 0 {
				offset = 0
			}

			a.netMu.Lock()
			page, err := sess.Messages(f.Name, 100, offset)
			a.netMu.Unlock()
			if err != nil {
				a.mu.Lock()
				if f.Name == a.folder {
					a.hasMore = false
				} else if ce, ok := a.cache[f.Name]; ok && ce != nil {
					ce.hasMore = false
				} else {
					a.cache[f.Name] = &folderCache{msgs: nil, loadedLimit: 0, hasMore: false, sel: -1, scrollY: 0}
				}
				a.mu.Unlock()
				continue
			}
			for i := range page {
				page[i].Folder = f.Name
			}

			a.mu.Lock()
			if a.fetchGen != gen {
				a.mu.Unlock()
				return
			}
			if f.Name == a.folder {
				existing := map[imap.UID]bool{}
				for _, m := range a.msgs {
					existing[m.UID] = true
				}
				for _, m := range page {
					if !existing[m.UID] {
						a.msgs = append(a.msgs, m)
						existing[m.UID] = true
					}
				}
				a.loadedLimit = len(a.msgs)
				a.hasMore = len(page) >= 100
			} else {
				ce, ok := a.cache[f.Name]
				if !ok || ce == nil {
					ce = &folderCache{msgs: []Message{}, loadedLimit: 0, hasMore: true, sel: -1, scrollY: 0}
					a.cache[f.Name] = ce
				}
				existing := map[imap.UID]bool{}
				for _, m := range ce.msgs {
					existing[m.UID] = true
				}
				for _, m := range page {
					if !existing[m.UID] {
						ce.msgs = append(ce.msgs, m)
						existing[m.UID] = true
					}
				}
				ce.loadedLimit = len(ce.msgs)
				ce.hasMore = len(page) >= 100
				ce.sel = firstUnread(ce.msgs)
			}
			a.mu.Unlock()
			madeProgress = true
			a.saveDiskCache()
			a.invalidateAll()
		}

		if !madeProgress {
			a.mu.Lock()
			a.bgFetching = false
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
	}
}

func (a *App) filterDropdown(c *ui.Context, t *ui.Theme, anchor ui.Element) {
	target := float32(0)
	if a.filterPanelOpen {
		target = 1
	}
	panelAnim := anchor.Animate("filter-panel", target, 140*time.Millisecond)
	if !a.filterPanelOpen {
		a.filterDateFromOpen = false
		a.filterDateToOpen = false
	}
	// A press outside the date picker closes it but leaves the filter panel
	// open; the picker is only a part of the filter.
	hadDateOpen := a.filterDateFromOpen || a.filterDateToOpen
	wasPanelOpen := a.filterPanelOpen
	// Build suggestion lists.
	fromSet := map[string]bool{}
	var fromOpts []string
	for _, m := range a.msgs {
		for _, v := range []string{m.FromName, m.FromAddr} {
			v = strings.TrimSpace(v)
			if v != "" && !fromSet[v] {
				fromSet[v] = true
				fromOpts = append(fromOpts, v)
			}
		}
	}
	var toOpts []string
	if a.folder == "SHARED" {
		for _, acc := range a.accounts {
			toOpts = append(toOpts, acc.Email)
		}
	} else {
		toSet := map[string]bool{}
		for _, m := range a.msgs {
			for _, v := range m.To {
				v = strings.TrimSpace(v)
				if v != "" && !toSet[v] {
					toSet[v] = true
					toOpts = append(toOpts, v)
				}
			}
		}
	}
	ui.PopoverBase(c, anchor, &a.filterPanelOpen, func(panel ui.Element) {
		panel.Padding(10).Background(t.Surface).Border(1, t.Border).Radius(10).
			Shadow(0, 12, 30, 0, ui.RGBA(0, 0, 0, 0.28)).MinWidth(280).
			Opacity(panelAnim).Translate(0, (1-panelAnim)*-6)
		panel.Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 10).Children(func() {
					flagged := a.filterFlagged
					if ui.Checkbox(c, &flagged, "Flagged").Changed() {
						a.filterFlagged = flagged
						a.invalidate()
					}
					unread := a.filterUnread
					if ui.Checkbox(c, &unread, "Unread").Changed() {
						a.filterUnread = unread
						a.invalidate()
					}
				})
				ui.Text(c, "From").FontSize(11).TextColor(t.TextMuted)
				cb := ui.ComboboxBase(c, &a.filterFrom)
				cb.Input.Height(28).FontSize(11).Padding(4, 8).Border(1, t.Border).Background(t.Surface).Radius(6)
				cb.Popup(func(p ui.Element) {
					p.Padding(6, 8).Background(t.SurfaceHover).Border(1, t.Border).Radius(8).MaxHeight(130)
					count := 0
					for _, opt := range fromOpts {
						if count >= 5 {
							break
						}
						if a.filterFrom != "" && !strings.Contains(strings.ToLower(opt), strings.ToLower(a.filterFrom)) {
							continue
						}
						count++
						item := cb.Item(opt).Padding(4, 8).Radius(6)
						if item.Highlighted() || item.Hovered() {
							item.Background(t.SurfaceHover)
						}
						item.Children(func() { ui.Text(c, opt).FontSize(11).SingleLine() })
					}
				})
				if f, ok := cb.Chosen(); ok {
					a.filterFrom = f
					a.invalidate()
				}
				if a.folder == "SHARED" {
					ui.Text(c, "To (account)").FontSize(11).TextColor(t.TextMuted)
					ui.Combobox(c, &a.filterTo, toOpts).Height(26).FontSize(11)
				} else {
					ui.Text(c, "To").FontSize(11).TextColor(t.TextMuted)
					cb2 := ui.ComboboxBase(c, &a.filterTo)
					cb2.Input.Height(28).FontSize(11).Padding(4, 8).Border(1, t.Border).Background(t.Surface).Radius(6)
					cb2.Popup(func(p ui.Element) {
						p.Padding(6, 8).Background(t.SurfaceHover).Border(1, t.Border).Radius(8).MaxHeight(130)
						count := 0
						for _, opt := range toOpts {
							if count >= 5 {
								break
							}
							if a.filterTo != "" && !strings.Contains(strings.ToLower(opt), strings.ToLower(a.filterTo)) {
								continue
							}
							count++
							item := cb2.Item(opt).Padding(4, 8).Radius(6)
							if item.Highlighted() || item.Hovered() {
								item.Background(t.SurfaceHover)
							}
							item.Children(func() { ui.Text(c, opt).FontSize(11).SingleLine() })
						}
					})
					if f, ok := cb2.Chosen(); ok {
						a.filterTo = f
						a.invalidate()
					}
				}
				a.dateFilterField(c, t, "After", &a.filterDateFrom, &a.filterDateFromOpen, "dateFrom")
				a.dateFilterField(c, t, "Before", &a.filterDateTo, &a.filterDateToOpen, "dateTo")
				if ui.Button(c.Key("clearFilters"), "Clear filters").Padding(4, 8).Radius(6).Clicked() {
					a.filterFlagged = false
					a.filterUnread = false
					a.filterFrom = ""
					a.filterTo = ""
					a.filterDateFrom = ""
					a.filterDateTo = ""
					a.filterDateFromOpen = false
					a.filterDateToOpen = false
					a.invalidate()
				}
			})
		})
	})
	if wasPanelOpen && hadDateOpen && !a.filterPanelOpen {
		a.filterPanelOpen = true
	}
}

// dateFilterField shows a typeable date and a calendar button that opens a
// date picker popover anchored to the field.
func (a *App) dateFilterField(c *ui.Context, t *ui.Theme, label string, value *string, open *bool, key string) {
	ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
	row := ui.Row(c).Gap(4).AlignItems(ui.Center)
	row.Children(func() {
		in := ui.TextInput(c, value).Placeholder("YYYY-MM-DD").Height(28).Grow(1).FontSize(11).Border(1, t.Border).Background(t.Surface).Radius(6)
		if in.Changed() {
			if *open {
				if d, ok := parseFilterDate(*value); ok {
					a.dateCursor = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
					a.dateCursorKey = key
				}
			}
			a.invalidate()
		}
		if in.Clicked() {
			if !*open {
				a.focusDateCursor(key, *value)
			}
			*open = true
		}
		calBtn := ui.ButtonBase(c.Key(key + "Cal")).Padding(4).Radius(6)
		calBtn.Children(func() { ui.Icon(c, iconCalendar).FontSize(14).TextColor(t.TextMuted) })
		if calBtn.Hovered() {
			calBtn.Background(t.SurfaceHover)
		}
		if calBtn.Clicked() {
			if *open {
				*open = false
			} else {
				a.focusDateCursor(key, *value)
				*open = true
			}
		}
	})
	ui.PopoverBase(c, row, open, func(panel ui.Element) {
		a.datePickerPanel(c, t, panel, value, open, key)
	})
}

// parseFilterDate parses a YYYY-MM-DD filter value.
func parseFilterDate(s string) (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// focusDateCursor points the date picker at the month of the current value.
func (a *App) focusDateCursor(key, value string) {
	d, ok := parseFilterDate(value)
	if !ok {
		d = time.Now()
	}
	a.dateCursor = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	a.dateCursorKey = key
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// datePickerPanel is the calendar that a date field opens: a header, year
// navigation, the month grid, and quick actions.
func (a *App) datePickerPanel(c *ui.Context, t *ui.Theme, panel ui.Element, value *string, open *bool, key string) {
	panel.Padding(12).Background(t.Surface).Border(1, t.Border).Radius(12).
		Shadow(0, 16, 40, 0, ui.RGBA(0, 0, 0, 0.32)).MinWidth(272).Gap(8)
	panel.Children(func() {
		selected, hasSelected := parseFilterDate(*value)
		if !hasSelected {
			selected = time.Now()
		}
		selected = time.Date(selected.Year(), selected.Month(), selected.Day(), 0, 0, 0, 0, time.Local)

		month := a.dateCursor
		if a.dateCursorKey != key || month.IsZero() {
			month = time.Date(selected.Year(), selected.Month(), 1, 0, 0, 0, 0, time.Local)
		}
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)

		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, "Choose date").FontSize(12).FontWeight(600).TextColor(t.Text)
			closeBtn := ui.ButtonBase(c.Key(key + "Close")).Padding(2).Radius(5)
			closeBtn.Children(func() { ui.Icon(c, iconX).FontSize(12).TextColor(t.TextMuted) })
			if closeBtn.Hovered() {
				closeBtn.Background(t.SurfaceHover)
			}
			if closeBtn.Clicked() {
				*open = false
			}
		})

		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Year").FontSize(10).TextColor(t.TextMuted)
			prev := ui.ButtonBase(c.Key(key+"PrevYear")).Padding(2, 8).Radius(6)
			prev.Children(func() { ui.Text(c, "‹").FontSize(15).TextColor(t.Text) })
			if prev.Hovered() {
				prev.Background(t.SurfaceHover)
			}
			if prev.Clicked() {
				month = month.AddDate(-1, 0, 0)
				a.dateCursor = month
				a.invalidate()
			}
			ui.Text(c, strconv.Itoa(month.Year())).Width(44).TextAlign(ui.Center).FontSize(11).FontWeight(600).TextColor(t.Text)
			next := ui.ButtonBase(c.Key(key+"NextYear")).Padding(2, 8).Radius(6)
			next.Children(func() { ui.Text(c, "›").FontSize(15).TextColor(t.Text) })
			if next.Hovered() {
				next.Background(t.SurfaceHover)
			}
			if next.Clicked() {
				month = month.AddDate(1, 0, 0)
				a.dateCursor = month
				a.invalidate()
			}
		})

		a.monthCalendar(c, t, selected, hasSelected, month, key, func(day time.Time) {
			*value = day.Format("2006-01-02")
			*open = false
			a.invalidate()
		})

		ui.Divider(c)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			if ui.Button(c.Key(key+"Today"), "Today").Padding(4, 10).Radius(6).FontSize(11).Clicked() {
				*value = time.Now().Format("2006-01-02")
				*open = false
				a.invalidate()
			}
			if ui.Button(c.Key(key+"Clear"), "Clear").Padding(4, 10).Radius(6).FontSize(11).
				Background(t.Surface).Border(1, t.Border).Clicked() {
				*value = ""
				*open = false
				a.invalidate()
			}
		})
	})
}

// monthCalendar renders a month grid where the arrows only move the displayed
// month. A day is committed only when the user clicks it.
func (a *App) monthCalendar(c *ui.Context, t *ui.Theme, selected time.Time, hasSelected bool, month time.Time, key string, choose func(time.Time)) {
	month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)

	ui.Column(c).Gap(2).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			prev := ui.ButtonBase(c.Key(key+"PrevMonth")).Padding(2, 8).Radius(6)
			prev.Children(func() { ui.Text(c, "‹").FontSize(15).TextColor(t.Text) })
			if prev.Hovered() {
				prev.Background(t.SurfaceHover)
			}
			if prev.Clicked() {
				month = month.AddDate(0, -1, 0)
				a.dateCursor = month
				a.invalidate()
			}
			ui.Text(c, month.Format("January 2006")).Grow(1).TextAlign(ui.Center).FontSize(11).FontWeight(600).TextColor(t.Text)
			next := ui.ButtonBase(c.Key(key+"NextMonth")).Padding(2, 8).Radius(6)
			next.Children(func() { ui.Text(c, "›").FontSize(15).TextColor(t.Text) })
			if next.Hovered() {
				next.Background(t.SurfaceHover)
			}
			if next.Clicked() {
				month = month.AddDate(0, 1, 0)
				a.dateCursor = month
				a.invalidate()
			}
		})

		ui.Row(c).Gap(1).Justify(ui.Center).Children(func() {
			for _, wd := range []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"} {
				ui.Text(c, wd).Width(32).TextAlign(ui.Center).FontSize(9).TextColor(t.TextMuted)
			}
		})

		today := time.Now()
		start := month.AddDate(0, 0, -int(month.Weekday()))
		for w := 0; w < 6; w++ {
			ui.Row(c).Gap(1).Justify(ui.Center).Children(func() {
				for i := 0; i < 7; i++ {
					day := start.AddDate(0, 0, w*7+i)
					chosen := hasSelected && sameDate(day, selected)
					cell := ui.ButtonBase(c.Key(key+"Day"+day.Format("20060102"))).Size(32, 28).Radius(6)
					if chosen {
						cell.Background(t.Accent)
					}
					if !chosen && cell.Hovered() {
						cell.Background(t.SurfaceHover)
					}
					if !chosen && sameDate(day, today) {
						cell.Border(1, t.Accent)
					}
					if cell.Clicked() {
						choose(day)
					}
					fg := t.Text
					if day.Month() != month.Month() {
						fg = t.TextMuted
					}
					if chosen {
						fg = t.AccentText
					}
					cell.Children(func() {
						ui.Text(c, strconv.Itoa(day.Day())).FontSize(11).TextColor(fg)
					})
				}
			})
		}
	})
}

func iconButton(c *ui.Context, key string, ic *ui.SVG) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Padding(6).Radius(7)
	b.Children(func() {
		ui.Icon(c, ic).FontSize(15).TextColor(t.TextMuted)
	})
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	return b
}

func actionButton(c *ui.Context, key string, ic *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.Button(c.Key(key), "").Padding(6, 10).Radius(8).Background(t.Surface).Border(1, t.Border)
	b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, ic).FontSize(14).TextColor(t.Text)
			ui.Text(c, label).FontSize(12).FontWeight(500).TextColor(t.Text)
		})
	})
	return b
}

// ------------------------------------------------------------ persisted ---

type diskCache struct {
	Accounts map[string]map[string][]Message `json:"accounts"`
}

type configFile struct {
	Accounts      []Account     `json:"accounts"`
	Active        int           `json:"active"`
	Theme         ThemeSettings `json:"theme"`
	CustomThemes  []CustomTheme `json:"customThemes,omitempty"`
	SharedMailbox bool          `json:"sharedMailbox,omitempty"`
	FolderOrder   []string      `json:"folderOrder,omitempty"`
	// legacy single-account format
	Account Account `json:"account"`
}

func configPath() string {
	if p := os.Getenv("GO_MAIL_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, appDataDir, "config.json")
}

func (a *App) saveConfig() {
	path := configPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if a.connected && a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active] = a.acct
		a.accounts[a.active].Theme = a.themeSettings
	}
	if a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active].Theme = a.themeSettings
	}
	// Never store passwords in plain text on disk.
	savedAccounts := append([]Account(nil), a.accounts...)
	for i := range savedAccounts {
		savedAccounts[i].Password = encryptSecret(savedAccounts[i].Password)
		savedAccounts[i].SignatureImage = ""
	}
	data, err := json.MarshalIndent(configFile{
		Accounts:      savedAccounts,
		Active:        a.active,
		Theme:         a.themeSettings,
		CustomThemes:  a.customThemes,
		SharedMailbox: a.sharedMailbox,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func (a *App) cachePath() string {
	if p := os.Getenv("GO_MAIL_CACHE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, appDataDir, "cache.json")
}

func (a *App) cacheSize() int64 {
	path := a.cachePath()
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func (a *App) clearDiskCache() {
	if path := a.cachePath(); path != "" {
		_ = os.Remove(path)
	}
	a.mu.Lock()
	a.diskCache = diskCache{Accounts: map[string]map[string][]Message{}}
	a.cache = map[string]*folderCache{}
	a.mu.Unlock()
	a.invalidateAll()
}

func (a *App) loadDiskCache() {
	path := a.cachePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		a.diskCache = diskCache{Accounts: map[string]map[string][]Message{}}
		return
	}
	var dc diskCache
	if json.Unmarshal(data, &dc) != nil {
		a.diskCache = diskCache{Accounts: map[string]map[string][]Message{}}
		return
	}
	if dc.Accounts == nil {
		dc.Accounts = map[string]map[string][]Message{}
	}
	a.diskCache = dc
}

// messageCacheKey names the account and physical folder a listed message came
// from. Shared-mailbox rows carry the account address in Folder; flagged rows
// carry the folder they were found in.
func (a *App) messageCacheKey(m Message) (email, folder string) {
	email = a.acct.Email
	folder = m.Folder
	if strings.Contains(m.Folder, "@") {
		email = m.Folder
		folder = "INBOX"
	}
	if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
		folder = a.folder
	}
	if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
		folder = "INBOX"
	}
	return email, folder
}

func updateCachedMessage(msgs []Message, uid imap.UID, folder string, fn func(*Message)) bool {
	for i := range msgs {
		if msgs[i].UID == uid && (msgs[i].Folder == folder || msgs[i].Folder == "") {
			fn(&msgs[i])
			return true
		}
	}
	return false
}

func removeCachedMessage(msgs []Message, uid imap.UID, folder string) []Message {
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.UID == uid && (m.Folder == folder || m.Folder == "") {
			continue
		}
		out = append(out, m)
	}
	return out
}

func addCachedMessage(msgs []Message, m Message) []Message {
	out := make([]Message, 0, len(msgs)+1)
	out = append(out, m)
	for _, x := range msgs {
		if x.UID == m.UID && x.Folder == m.Folder {
			continue
		}
		out = append(out, x)
	}
	return out
}

// cacheUpdateMessageAt updates every cached copy of a message after a flag or
// read-state change.
func (a *App) cacheUpdateMessageAt(email, folder string, m Message, fn func(*Message)) {
	if email == "" || folder == "" {
		return
	}
	a.mu.Lock()
	if acc := a.diskCache.Accounts[email]; acc != nil {
		updateCachedMessage(acc[folder], m.UID, folder, fn)
	}
	if email == a.acct.Email {
		if ce := a.cache[folder]; ce != nil {
			updateCachedMessage(ce.msgs, m.UID, folder, fn)
		}
		if ce := a.cache["FLAGGED"]; ce != nil {
			updateCachedMessage(ce.msgs, m.UID, m.Folder, fn)
			if folder != m.Folder {
				updateCachedMessage(ce.msgs, m.UID, folder, fn)
			}
		}
	}
	if ce := a.cache["SHARED"]; ce != nil {
		updateCachedMessage(ce.msgs, m.UID, email, fn)
	}
	a.mu.Unlock()
}

// cacheRemoveMessageAt drops a moved or deleted message from its old folder
// and from the virtual views that may still list it.
func (a *App) cacheRemoveMessageAt(email, folder string, m Message) {
	if email == "" || folder == "" {
		return
	}
	a.mu.Lock()
	if acc := a.diskCache.Accounts[email]; acc != nil {
		acc[folder] = removeCachedMessage(acc[folder], m.UID, folder)
	}
	if email == a.acct.Email {
		if ce := a.cache[folder]; ce != nil {
			ce.msgs = removeCachedMessage(ce.msgs, m.UID, folder)
			ce.loadedLimit = len(ce.msgs)
		}
		if ce := a.cache["FLAGGED"]; ce != nil {
			ce.msgs = removeCachedMessage(ce.msgs, m.UID, m.Folder)
			if folder != m.Folder {
				ce.msgs = removeCachedMessage(ce.msgs, m.UID, folder)
			}
			ce.loadedLimit = len(ce.msgs)
		}
	}
	if ce := a.cache["SHARED"]; ce != nil {
		ce.msgs = removeCachedMessage(ce.msgs, m.UID, email)
		ce.loadedLimit = len(ce.msgs)
	}
	a.mu.Unlock()
}

// cacheAddMessageAt puts a message into the cache for its destination folder
// so switching to that folder does not need a round-trip.
func (a *App) cacheAddMessageAt(email, folder string, m Message) {
	if email == "" || folder == "" {
		return
	}
	m.Folder = folder
	a.mu.Lock()
	if a.diskCache.Accounts == nil {
		a.diskCache.Accounts = map[string]map[string][]Message{}
	}
	if a.diskCache.Accounts[email] == nil {
		a.diskCache.Accounts[email] = map[string][]Message{}
	}
	a.diskCache.Accounts[email][folder] = addCachedMessage(a.diskCache.Accounts[email][folder], m)
	if email == a.acct.Email {
		if ce := a.cache[folder]; ce != nil {
			ce.msgs = addCachedMessage(ce.msgs, m)
			ce.loadedLimit = len(ce.msgs)
		}
	}
	if folder == "INBOX" {
		if ce := a.cache["SHARED"]; ce != nil {
			shared := m
			shared.Folder = email
			ce.msgs = addCachedMessage(ce.msgs, shared)
			ce.loadedLimit = len(ce.msgs)
		}
	}
	if m.Flagged && email == a.acct.Email {
		if ce := a.cache["FLAGGED"]; ce != nil {
			ce.msgs = addCachedMessage(ce.msgs, m)
			ce.loadedLimit = len(ce.msgs)
		}
	}
	a.mu.Unlock()
}

func (a *App) cacheAddToFlagged(email string, m Message) {
	if email != a.acct.Email {
		return
	}
	a.mu.Lock()
	if ce := a.cache["FLAGGED"]; ce != nil {
		ce.msgs = addCachedMessage(ce.msgs, m)
		ce.loadedLimit = len(ce.msgs)
	}
	a.mu.Unlock()
}

func (a *App) cacheRemoveFromFlagged(email string, m Message) {
	if email != a.acct.Email {
		return
	}
	a.mu.Lock()
	if ce := a.cache["FLAGGED"]; ce != nil {
		ce.msgs = removeCachedMessage(ce.msgs, m.UID, m.Folder)
		ce.loadedLimit = len(ce.msgs)
	}
	a.mu.Unlock()
}

func (a *App) recordMove(email, from string, m Message, to string) {
	a.cacheRemoveMessageAt(email, from, m)
	if to != "" {
		m.Folder = to
		a.cacheAddMessageAt(email, to, m)
	}
	a.saveDiskCache()
}

func (a *App) saveDiskCache() {
	path := a.cachePath()
	if path == "" {
		return
	}
	a.mu.Lock()
	if a.diskCache.Accounts == nil {
		a.diskCache.Accounts = map[string]map[string][]Message{}
	}
	if a.acct.Email != "" {
		if a.diskCache.Accounts[a.acct.Email] == nil {
			a.diskCache.Accounts[a.acct.Email] = map[string][]Message{}
		}
		if a.folder != "" && a.folder != "SHARED" && a.folder != "FLAGGED" {
			a.diskCache.Accounts[a.acct.Email][a.folder] = append([]Message(nil), a.msgs...)
		}
		for name, ce := range a.cache {
			// A real active folder is saved from a.msgs above. Its old cache
			// entry can be a stale slice after a cache-hit switch.
			if ce != nil && !(name == a.folder && name != "SHARED" && name != "FLAGGED") {
				a.diskCache.Accounts[a.acct.Email][name] = append([]Message(nil), ce.msgs...)
			}
		}
	}
	// Snapshot while holding the lock, then marshal outside it so a large
	// cache cannot stall the UI thread.
	snapshot := diskCache{Accounts: make(map[string]map[string][]Message, len(a.diskCache.Accounts))}
	for email, folders := range a.diskCache.Accounts {
		copied := make(map[string][]Message, len(folders))
		for name, msgs := range folders {
			copied[name] = append([]Message(nil), msgs...)
		}
		snapshot.Accounts[email] = copied
	}
	a.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func (a *App) loadConfig() {
	path := configPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg configFile
	if json.Unmarshal(data, &cfg) != nil {
		return
	}
	if len(cfg.Accounts) == 0 && cfg.Account.Email != "" {
		cfg.Accounts = []Account{cfg.Account}
		cfg.Active = 0
	}
	for i := range cfg.Accounts {
		cfg.Accounts[i].Password = decryptSecret(cfg.Accounts[i].Password)
	}
	if cfg.Account.Email != "" {
		cfg.Account.Password = decryptSecret(cfg.Account.Password)
	}
	a.accounts = cfg.Accounts
	for i := range a.accounts {
		acct := &a.accounts[i]
		if len(acct.SignatureCanvas) == 0 {
			imgs := acct.SignatureImages
			if len(imgs) == 0 && acct.SignatureImage != "" {
				imgs = []string{acct.SignatureImage}
			}
			for j, b64 := range imgs {
				acct.SignatureCanvas = append(acct.SignatureCanvas, SignatureImage{
					Data:  b64,
					X:     6 + float32(j%3)*7,
					Y:     6 + float32(j%4)*6,
					Width: 22,
				})
			}
			acct.SignatureImages = nil
			acct.SignatureImage = ""
		}
	}
	if cfg.Active < 0 || cfg.Active >= len(a.accounts) {
		cfg.Active = 0
	}
	a.active = cfg.Active
	a.customThemes = cfg.CustomThemes
	if a.active >= 0 && a.active < len(a.accounts) && a.accounts[a.active].Theme.Theme != "" {
		a.themeSettings = a.accounts[a.active].Theme
	} else {
		a.themeSettings = cfg.Theme
	}
	a.sharedMailbox = cfg.SharedMailbox
	if len(cfg.FolderOrder) > 0 && a.active >= 0 && a.active < len(a.accounts) {
		a.accounts[a.active].FolderOrder = cfg.FolderOrder
	}
	a.loadDiskCache()
	// Migrate any legacy plain-text passwords to encrypted storage.
	a.saveConfig()
	// Legacy configs stored the theme in "preset"; even older ones stored an
	// accent id there. Preserve the accent and fall back to the default theme.
	if a.themeSettings.Theme == "" && a.themeSettings.Preset != "" {
		if a.themeSettings.Preset == "system" || isPresetTheme(a.themeSettings.Preset) || customThemeByID(a.customThemes, a.themeSettings.Preset) != nil {
			a.themeSettings.Theme = a.themeSettings.Preset
		} else {
			if a.themeSettings.Accent == "" {
				a.themeSettings.Accent = a.themeSettings.Preset
			}
			a.themeSettings.Theme = "nocturne"
		}
	}
	if a.themeSettings.Theme == "" {
		a.themeSettings.Theme = "nocturne"
	}
	if a.themeSettings.FontSize == 0 {
		a.themeSettings.FontSize = 14
	}
	if a.themeSettings.Radius == 0 {
		a.themeSettings.Radius = 9
	}
	a.palette = resolvePalette(a.themeSettings, a.systemDark, a.customThemes)
	activePalette = a.palette
	a.dark = a.palette.Dark
	if len(a.accounts) > 0 {
		a.form = a.accounts[a.active]
		a.acct = a.accounts[a.active]
	}
}

func crashLogPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return "crash.log"
	}
	return filepath.Join(dir, appDataDir, "crash.log")
}

func logPanic(where string, r any) {
	path := crashLogPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "panic in %s: %v\n", where, r)
	f.Write(debug.Stack())
	f.WriteString("\n")
}
