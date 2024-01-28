package main

import (
	"embed"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-autostart"
	"github.com/fsnotify/fsnotify"
	notify "github.com/getlantern/notifier"
	"github.com/getlantern/systray"
	"github.com/sagernet/sing/common/json"
	"github.com/skratchdot/open-golang/open"
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
var ConfDir = ""

var sb *SingBox

var confFileReg = regexp.MustCompile(`^config(\.\w+)?.json$`)

type AppConf struct {
	LaunchdAtLogin bool
	ProxyAtLogin   bool
	ActiveConfig   string
}

var appConf = &AppConf{
	ActiveConfig: "config.json",
}

type menu struct {
	Title   string
	Tips    string
	Icon    []byte
	OnClick func(m *systray.MenuItem)
}

func main() {
	defer func() {
		sb.Close()
	}()
	home, _ = os.UserHomeDir()
	_ = SaveDir(singbox, home, false)

	loadAppConf()
	ConfDir = filepath.Join(home, ".singbox")

	os.Chdir(ConfDir)

	Conf = filepath.Join(home, ".singbox", "config.json")

	systray.Run(onReady, onExit)
}

func onReady() {
	_icon := icon
	_iconOff := iconOff

	if isWin() {
		_icon = iconWin
		_iconOff = iconOffWin
	}

	systray.SetTemplateIcon(_iconOff, _iconOff)
	sb = &SingBox{ConfPath: Conf}

	_startProxy := func(m *systray.MenuItem) {
		err := sb.Start(ConfDir, appConf.ActiveConfig)
		if err != nil {
			notice(&notify.Notification{
				Title:   "SingBox Start error",
				Message: err.Error(),
			})
			return
		}
		m.SetTitle("StopProxy")
		systray.SetTemplateIcon(_icon, _icon)
	}

	var restartMenu *systray.MenuItem

	startProxy := func(m *systray.MenuItem) {
		err := sb.Start(ConfDir, appConf.ActiveConfig)
		if err != nil {
			if strings.Contains(err.Error(), "configure tun interface") {
				if strings.Contains(err.Error(), "Access is denied") {
					notice2("when tun mod, please run app as admin")
					return
				}
				if strings.Contains(err.Error(), "operation not permitted") {
					_ = runAsAdministrator(func() {
						sb.Close()
					})
				}
			}
			if strings.Contains(err.Error(), "no route to internet") {
				go func() {
					time.Sleep(time.Second * 10)
					_startProxy(m)
				}()
			} else {
				notice(&notify.Notification{
					Title:   "SingBox Config ERR",
					Message: err.Error(),
				})
			}
			return
		}
		m.SetTitle("StopProxy")
		systray.SetTemplateIcon(_icon, _icon)
		if restartMenu != nil {
			restartMenu.Show()
		}
	}

	closeProxy := func(m *systray.MenuItem) {
		sb.Close()
		m.SetTitle("StartProxy")
		systray.SetTemplateIcon(_iconOff, _iconOff)

		if restartMenu != nil {
			restartMenu.Hide()
		}
	}

	proxyMenu := addMenu(&menu{
		Title: "StartProxy",
		OnClick: func(m *systray.MenuItem) {
			m.Disable()
			if sb.Running {
				closeProxy(m)
			} else {
				startProxy(m)
			}
			m.Enable()
		},
	})

	if appConf.ProxyAtLogin {
		proxyMenu.Disable()
		startProxy(proxyMenu)
		proxyMenu.Enable()
	}

	restartProxy := func() {
		closeProxy(proxyMenu)
		startProxy(proxyMenu)
	}

	restartMenu = addMenu(&menu{
		Title: "RestartProxy",
		OnClick: func(m *systray.MenuItem) {
			if sb.Running {
				restartProxy()
			} else {
				startProxy(proxyMenu)
			}
		},
	})

	options, err := readConfig(filepath.Join(ConfDir, appConf.ActiveConfig))

	if err == nil {
		addMenu(&menu{
			Title: "Dashboard",
			OnClick: func(m *systray.MenuItem) {
				_ = open.Run(fmt.Sprintf("http://%s/ui", options.Experimental.ClashAPI.ExternalController))
			},
		})
	}

	addCheckboxMenu(&menu{
		Title: "LaunchdAtLogin",
		OnClick: func(m *systray.MenuItem) {
			app := &autostart.App{
				Name:        "singbox",
				DisplayName: "SingBox",
				Exec:        []string{"open", "-a", "SingBox"},
			}

			if isWin() {
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
			saveAppConf()
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
			saveAppConf()
		},
	}, appConf.ProxyAtLogin)

	addMenu(&menu{
		Title: "EditConfig",
		OnClick: func(m *systray.MenuItem) {
			confFile := filepath.Join(ConfDir, appConf.ActiveConfig)
			err := open.RunWith(confFile, "Visual Studio Code")
			if err != nil {
				_ = open.Run(ConfDir)
			}
		},
	})

	var confList []*menu

	if files, err := dirFileList(ConfDir); err == nil {
		for _, v := range files {
			fileName := v
			if confFileReg.MatchString(fileName) == false {
				continue
			}
			confList = append(confList, &menu{
				Title: fileName,
				OnClick: func(m *systray.MenuItem) {
					if fileName == appConf.ActiveConfig {
						return
					}
					appConf.ActiveConfig = fileName
					saveAppConf()
					fmt.Println(appConf.ActiveConfig)
					restartProxy()
				},
			})
		}
	}

	addRadioMenu("SelectConfig", appConf.ActiveConfig, confList)

	addMenuGroup("About", []*menu{
		{
			Title: "SingBox",
			OnClick: func(m *systray.MenuItem) {
				_ = open.Run("http://github.com/daodao97/SingBox")
			},
		},
		{
			Title: "sing-box docs",
			OnClick: func(m *systray.MenuItem) {
				_ = open.Run("https://sing-box.sagernet.org/zh/configuration/")
			},
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
		notice2("file watcher err " + err.Error())
		return
	}
	defer watcher.Close()

	if files, err := dirFileList(ConfDir); err == nil {
		for _, v := range files {
			if strings.Contains(v, ".json") {
				err = watcher.Add(filepath.Join(ConfDir, v))
				if err != nil {
					notice2("file watcher err " + err.Error())
				}
			}
		}
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			fmt.Println("event:", event, fmt.Sprintf("%v", sb.Running))
			if filepath.Join(ConfDir, appConf.ActiveConfig) == event.Name && strings.Contains(event.String(), "WRITE") && sb.Running {
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
}

func notice(msg *notify.Notification) {
	msg.Sender = "run.daodao.SingBox"
	n := notify.NewNotifications()
	err := n.Notify(msg)
	if err != nil {
		fmt.Println("notify err", err)
	}
}

func notice2(message string) {
	n := notify.NewNotifications()
	err := n.Notify(&notify.Notification{Title: "SingBox", Message: message, Sender: "run.daodao.SingBox"})
	if err != nil {
		fmt.Println("notify err", err)
	}
}

func saveAppConf() {
	j, _ := json.Marshal(appConf)
	err := saveFile(filepath.Join(home, ".singbox", "app.json"), j)
	if err != nil {
		log.Println("save app.json err", err)
	}
}

func loadAppConf() {
	bt, err := readFile(filepath.Join(home, ".singbox", "app.json"))
	if err == nil {
		err = json.Unmarshal(bt, appConf)
	} else {
		log.Println("read app.json err", err)
	}
	if appConf.ActiveConfig == "" {
		appConf.ActiveConfig = "config.json"
	}
}
