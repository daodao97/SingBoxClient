package tun

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"strconv"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/abx"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/fsnotify/fsnotify"
)

type packageManager struct {
	callback        PackageManagerCallback
	watcher         *fsnotify.Watcher
	idByPackage     map[string]uint32
	sharedByPackage map[string]uint32
	packageById     map[uint32]string
	sharedById      map[uint32]string
}

func NewPackageManager(callback PackageManagerCallback) (PackageManager, error) {
	return &packageManager{callback: callback}, nil
}

func (m *packageManager) Start() error {
	err := m.updatePackages()
	if err != nil {
		return E.Cause(err, "read packages list")
	}
	err = m.startWatcher()
	if err != nil {
		m.callback.NewError(context.Background(), E.Cause(err, "create fsnotify watcher"))
	}
	return nil
}

func (m *packageManager) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add("/data/system/packages.xml")
	if err != nil {
		return err
	}
	m.watcher = watcher
	go m.loopUpdate()
	return nil
}

func (m *packageManager) loopUpdate() {
	for {
		select {
		case _, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			err := m.updatePackages()
			if err != nil {
				m.callback.NewError(context.Background(), E.Cause(err, "update packages"))
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.callback.NewError(context.Background(), E.Cause(err, "fsnotify error"))
		}
	}
}

func (m *packageManager) Close() error {
	return common.Close(common.PtrOrNil(m.watcher))
}

func (m *packageManager) IDByPackage(packageName string) (uint32, bool) {
	id, loaded := m.idByPackage[packageName]
	return id, loaded
}

func (m *packageManager) IDBySharedPackage(sharedPackage string) (uint32, bool) {
	id, loaded := m.sharedByPackage[sharedPackage]
	return id, loaded
}

func (m *packageManager) PackageByID(id uint32) (string, bool) {
	packageName, loaded := m.packageById[id]
	return packageName, loaded
}

func (m *packageManager) SharedPackageByID(id uint32) (string, bool) {
	sharedPackage, loaded := m.sharedById[id]
	return sharedPackage, loaded
}

func (m *packageManager) updatePackages() error {
	packagesData, err := os.ReadFile("/data/system/packages.xml")
	if err != nil {
		return err
	}
	var decoder *xml.Decoder
	reader, ok := abx.NewReader(packagesData)
	if ok {
		decoder = xml.NewTokenDecoder(reader)
	} else {
		decoder = xml.NewDecoder(bytes.NewReader(packagesData))
	}
	return m.decodePackages(decoder)
}

func (m *packageManager) decodePackages(decoder *xml.Decoder) error {
	idByPackage := make(map[string]uint32)
	sharedByPackage := make(map[string]uint32)
	packageById := make(map[uint32]string)
	sharedById := make(map[uint32]string)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		element, isStart := token.(xml.StartElement)
		if !isStart {
			continue
		}

		switch element.Name.Local {
		case "package":
			var name string
			var userID uint64
			for _, attr := range element.Attr {
				switch attr.Name.Local {
				case "name":
					name = attr.Value
				case "userId", "sharedUserId":
					userID, err = strconv.ParseUint(attr.Value, 10, 32)
					if err != nil {
						return err
					}
				}
			}
			if userID == 0 && name == "" {
				continue
			}
			idByPackage[name] = uint32(userID)
			packageById[uint32(userID)] = name
		case "shared-user":
			var name string
			var userID uint64
			for _, attr := range element.Attr {
				switch attr.Name.Local {
				case "name":
					name = attr.Value
				case "userId":
					userID, err = strconv.ParseUint(attr.Value, 10, 32)
					if err != nil {
						return err
					}
					packageById[uint32(userID)] = name
				}
			}
			if userID == 0 && name == "" {
				continue
			}
			sharedByPackage[name] = uint32(userID)
			sharedById[uint32(userID)] = name
		}
	}
	m.idByPackage = idByPackage
	m.sharedByPackage = sharedByPackage
	m.packageById = packageById
	m.sharedById = sharedById
	m.callback.OnPackagesUpdated(len(packageById), len(sharedById))
	return nil
}
