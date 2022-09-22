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
	"regexp"
	"runtime"
	"strings"
	"time"

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
	loadConf()

	ConfDir = filepath.Join(home, ".singbox")
	Conf = filepath.Join(home, ".singbox", "config.json")
	systray.Run(onReady, onExit)
}

func onReady() {
	_icon := icon
	_iconOff := iconOff

	if runtime.GOOS == "windows" {
		_icon = iconWin
		_iconOff = iconOffWin
	}

	systray.SetTemplateIcon(_iconOff, _iconOff)
	sb = &SingBox{ConfPath: Conf}

	_startProxy := func(m *systray.MenuItem) {
		err := sb.Start(filepath.Join(ConfDir, appConf.ActiveConfig))
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

	startProxy := func(m *systray.MenuItem) {
		err := sb.Start(filepath.Join(ConfDir, appConf.ActiveConfig))
		if err != nil {
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
			_ = open.Run("http://yacd.metacubex.one")
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
					saveConf()
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
		log.Fatal(err)
	}
	defer watcher.Close()

	if files, err := dirFileList(ConfDir); err == nil {
		for _, v := range files {
			err = watcher.Add(filepath.Join(ConfDir, v))
			if err != nil {
				fmt.Println(err)
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
	if appConf.ActiveConfig == "" {
		appConf.ActiveConfig = "config.json"
	}
}
