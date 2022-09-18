package main

import (
	"embed"
	_ "embed"
	"fmt"
	"github.com/sagernet/sing-box/common/json"
	"github.com/skratchdot/open-golang/open"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/emersion/go-autostart"
	"github.com/fsnotify/fsnotify"
	notify "github.com/getlantern/notifier"
	"github.com/getlantern/systray"
	"github.com/spf13/afero"
)

//go:embed icon/icon.png
var icon []byte

//go:embed icon/icon_off.png
var iconOff []byte

//go:embed icon/icon.ico
var iconWin []byte

//go:embed icon/icon_off.ico
var iconOffWin []byte

//go:embed icon/logo.png
var logo []byte

var home string

//go:embed .singbox
var singbox embed.FS

var Conf = ""

var sb *SingBox

type AppConf struct {
	LaunchdAtLogin bool
	ProxyAtLogin   bool
}

var appConf = &AppConf{}

func main() {
	defer func() {
		sb.Close()
	}()
	home, _ = os.UserHomeDir()
	_ = SaveDir(singbox, home, false)
	loadConf()

	Conf = filepath.Join(home, ".singbox", "config.json")
	systray.Run(onReady, onExit)
}

type menu struct {
	Title   string
	Tips    string
	Icon    []byte
	OnClick func(m *systray.MenuItem)
}

func addMenu(menu *menu) *systray.MenuItem {
	m := systray.AddMenuItem(menu.Title, menu.Tips)
	if len(menu.Icon) > 0 {
		m.SetIcon(menu.Icon)
	}
	go func() {
		for {
			select {
			case <-m.ClickedCh:
				menu.OnClick(m)
			}
		}
	}()

	return m
}

func addCheckboxMenu(menu *menu, checked bool) *systray.MenuItem {
	m := systray.AddMenuItemCheckbox(menu.Title, menu.Tips, checked)
	if len(menu.Icon) > 0 {
		m.SetIcon(menu.Icon)
	}
	go func() {
		for {
			select {
			case <-m.ClickedCh:
				menu.OnClick(m)
			}
		}
	}()

	return m
}

func onReady() {

	_icon := icon
	_iconOff := iconOff

	if runtime.GOOS == "windows" {
		_icon = iconWin
		_iconOff = iconOffWin
	}

	//systray.SetIcon(_iconOff)
	systray.SetTemplateIcon(_iconOff, _iconOff)
	sb = &SingBox{ConfPath: Conf}

	startProxy := func(m *systray.MenuItem) {
		err := sb.Start()
		if err != nil {
			notice(&notify.Notification{
				Title:   "SingBox Config ERR",
				Message: err.Error(),
			})
			return
		}
		m.SetTitle("StopProxy")

		//systray.SetIcon(_icon)
		systray.SetTemplateIcon(_icon, _icon)

	}

	closeProxy := func(m *systray.MenuItem) {
		sb.Close()
		m.SetTitle("StartProxy")
		systray.SetTemplateIcon(_iconOff, _iconOff)
	}

	proxyMenu := addMenu(&menu{
		Title: "StartProxy",
		OnClick: func(m *systray.MenuItem) {
			if sb.Running {
				closeProxy(m)
			} else {
				startProxy(m)
			}
		},
	})

	if appConf.ProxyAtLogin {
		startProxy(proxyMenu)
	}

	restartProxy := func() {
		closeProxy(proxyMenu)
		startProxy(proxyMenu)
	}

	addMenu(&menu{
		Title: "RestartProxy",
		OnClick: func(m *systray.MenuItem) {
			if sb.Running {
				restartProxy()
			} else {
				startProxy(proxyMenu)
			}
		},
	})

	addMenu(&menu{
		Title: "EditConfig",
		OnClick: func(m *systray.MenuItem) {
			_, err := exec.Command("code", Conf).Output()
			fmt.Println(err)
			if err != nil {
				_ = open.Run(filepath.Join(home, ".singbox"))
			}
		},
	})

	addCheckboxMenu(&menu{
		Title: "LaunchdAtLogin",
		OnClick: func(m *systray.MenuItem) {
			app := &autostart.App{
				Name:        "singbox",
				DisplayName: "SingBox",
				Exec:        []string{"open", "-a", "SingBox"},
			}

			if runtime.GOOS == "windows" {
				dir, _ := os.Getwd()
				app.Exec = []string{filepath.Join(dir, "SingBox.exe")}
			}

			if !m.Checked() {
				m.Check()
				if app.IsEnabled() {
					return
				}

				log.Println("Enabling app...")
				if err := app.Enable(); err != nil {
					log.Fatal(err)
				}

			} else {
				m.Uncheck()
				if !app.IsEnabled() {
					return
				}
				log.Println("App is already enabled, removing it...")

				if err := app.Disable(); err != nil {
					log.Fatal(err)
				}
			}

			log.Println("Done!")
			appConf.LaunchdAtLogin = m.Checked()
			saveConf()
		},
	}, appConf.LaunchdAtLogin)

	addCheckboxMenu(&menu{
		Title: "ProxyAtLogin",
		OnClick: func(m *systray.MenuItem) {
			if !m.Checked() {
				m.Check()

			} else {
				m.Uncheck()
			}
			appConf.ProxyAtLogin = m.Checked()
			saveConf()
		},
	}, appConf.ProxyAtLogin)

	addMenu(&menu{
		Title: "Dashboard",
		OnClick: func(m *systray.MenuItem) {
			_ = open.Run("http://yacd.metacubex.one/#/configs")
		},
	})

	addMenu(&menu{
		Title: "GithubStar",
		OnClick: func(m *systray.MenuItem) {
			_ = open.Run("http://github.com/daodao97/SingBox")
		},
	})

	systray.AddSeparator()
	addMenu(&menu{
		Title: "Quit",
		OnClick: func(m *systray.MenuItem) {
			sb.Close()
			systray.Quit()
		},
	})

	go watcher(sb, func(actType string) {
		if sb.Running == false {
			return
		}
		if actType == "@CONTENTCLICKED" || actType == "@ACTIONCLICKED" {
			restartProxy()
		}
	})
}

func onExit() {
	fmt.Println("exit app")
}

func watcher(sb *SingBox, cb func(actType string)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				fmt.Println("event:", event, fmt.Sprintf("%v", sb.Running))
				if strings.Contains(event.String(), "WRITE") && sb.Running {
					notice(&notify.Notification{
						Title:   "SingBox Config Change",
						Message: "Click me to reload SingBox",
						OnClick: cb,
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(Conf)
	if err != nil {
		fmt.Println(err)
	}

	<-make(chan struct{})
}

func notice(msg *notify.Notification) {
	msg.Sender = "run.daodao.SingBox"
	n := notify.NewNotifications()
	err := n.Notify(msg)
	if err != nil {
		fmt.Println("notify err", err)
	}
}

func saveConf() {
	appFS := afero.NewOsFs()
	j, _ := json.Marshal(appConf)
	err := afero.WriteFile(appFS, filepath.Join(home, ".singbox", "app.json"), j, 0644)
	if err != nil {
		log.Println("save app.json err", err)
	}
}

func loadConf() {
	appFS := afero.NewOsFs()
	bt, err := afero.ReadFile(appFS, filepath.Join(home, ".singbox", "app.json"))
	if err == nil {
		err = json.Unmarshal(bt, appConf)
	} else {
		log.Println("read app.json err", err)
	}
}
