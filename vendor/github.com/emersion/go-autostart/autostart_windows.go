package autostart

// #cgo LDFLAGS: -lole32 -luuid
/*
#define WIN32_LEAN_AND_MEAN
#include <stdint.h>
#include <windows.h>

uint64_t CreateShortcut(char *shortcutA, char *path, char *args);
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var startupDir string

func init() {
	startupDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
}

func (a *App) path() string {
	return filepath.Join(startupDir, a.Name+".lnk")
}

func (a *App) IsEnabled() bool {
	_, err := os.Stat(a.path())
	return err == nil
}

func (a *App) Enable() error {
	path := a.Exec[0]
	args := strings.Join(a.Exec[1:], " ")

	if err := os.MkdirAll(startupDir, 0777); err != nil {
		return err
	}
	res := C.CreateShortcut(C.CString(a.path()), C.CString(path), C.CString(args))
	if res != 0 {
		return errors.New(fmt.Sprintf("autostart: cannot create shortcut '%s' error code: 0x%.8x", a.path(), res))
	}
	return nil
}

func (a *App) Disable() error {
	return os.Remove(a.path())
}
