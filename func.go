package main

import (
	"embed"
	"github.com/getlantern/systray"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func SaveDir(efs embed.FS, dir string, overwrite bool) error {
	return fs.WalkDir(efs, ".", func(path string, d fs.DirEntry, err error) error {
		if path == "." {
			return nil
		}
		fullPath := filepath.Join(dir, path)
		_, err = os.Stat(fullPath)
		exist := true
		if err != nil {
			if os.IsNotExist(err) {
				exist = false
			} else {
				return errors.Wrap(err, "SaveDir.Stat")
			}
		}
		if d.IsDir() {
			if exist {
				return nil
			}
			err = os.Mkdir(fullPath, os.ModePerm)
			if err != nil {
				return errors.Wrap(err, "SaveDir.Mkdir")
			}
			return nil
		} else {
			if exist && !overwrite {
				return nil
			}
			// info, _ := os.Stat(path)
			f, _ := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0666))
			defer f.Close()
			content, err := efs.ReadFile(path)
			if err != nil {
				return errors.Wrap(err, "SaveDir.ReadFile")
			}
			_, err = f.Write(content)
			if err != nil {
				return errors.Wrap(err, "SaveDir.Write")
			}
			return nil
		}
	})
}

func dirFileList(dir string) ([]string, error) {
	var files []string
	list, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		files = append(files, v.Name())
	}
	return files, nil
}

func addRadioMenu(title string, defaultTitle string, sub []*menu) {
	boot := systray.AddMenuItem(title, "")
	var miArr []*systray.MenuItem
	for i, v := range sub {
		mi := boot.AddSubMenuItemCheckbox(v.Title, v.Title, v.Title == defaultTitle)
		_v := v
		_i := i
		miArr = append(miArr, mi)
		go func() {
			for {
				select {
				case <-mi.ClickedCh:
					_v.OnClick(mi)
					for j, e := range miArr {
						if j == _i {
							e.Check()
						} else {
							e.Uncheck()
						}
					}
				}
			}
		}()
	}
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

func addMenuGroup(title string, sub []*menu) {
	boot := systray.AddMenuItem(title, "")
	var miArr []*systray.MenuItem
	for _, v := range sub {
		mi := boot.AddSubMenuItem(v.Title, v.Title)
		_v := v
		miArr = append(miArr, mi)
		go func() {
			for {
				select {
				case <-mi.ClickedCh:
					_v.OnClick(mi)
				}
			}
		}()
	}
}

var _fs = afero.NewOsFs()

func readFile(path string) ([]byte, error) {
	return afero.ReadFile(_fs, path)
}

func saveFile(path string, content []byte) error {
	return afero.WriteFile(_fs, path, content, 0644)
}

func fileExist(path string) (bool, error) {
	return afero.Exists(_fs, path)
}

func isWin() bool {
	return runtime.GOOS == "windows"
}

func isMac() bool {
	return runtime.GOOS == "darwin"
}

func runAsAdministrator(cb func()) error {
	str := `do shell script "/Applications/SingBox.app/Contents/MacOS/sbox >/dev/null 2>&1 &" with prompt "开启增强模式" with administrator privileges`
	cmd := exec.Command("osascript", "-e", str)
	_, err := cmd.Output()
	if err != nil {
		return err
	}
	cb()
	os.Exit(0)
	return nil
}
