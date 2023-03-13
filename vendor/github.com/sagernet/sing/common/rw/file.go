package rw

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/sagernet/sing/common"
)

func FileExists(path string) bool {
	return common.Error(os.Stat(path)) == nil
}

func CopyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	if strings.Contains(dstPath, "/") {
		parent := dstPath[:strings.LastIndex(dstPath, "/")]
		if !FileExists(parent) {
			err = os.MkdirAll(parent, 0o755)
			if err != nil {
				return err
			}
		}
	}
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	return common.Error(io.Copy(dstFile, srcFile))
}

func WriteFile(path string, content []byte) error {
	if strings.Contains(path, "/") {
		parent := path[:strings.LastIndex(path, "/")]
		if !FileExists(parent) {
			err := os.MkdirAll(parent, 0o755)
			if err != nil {
				return err
			}
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(content)
	return err
}

func ReadJSON(path string, data any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(content, data)
	if err != nil {
		return err
	}
	return nil
}

func WriteJSON(path string, data any) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return WriteFile(path, content)
}
