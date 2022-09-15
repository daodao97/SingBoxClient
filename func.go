package main

import (
	"embed"
	"io/fs"
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
