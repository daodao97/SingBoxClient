package main

import (
	"embed"
	"github.com/getlantern/systray"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
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
