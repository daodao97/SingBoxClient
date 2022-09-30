package main

import (
	"context"
	"github.com/tidwall/jsonc"
	"log"
	"os"
	"path/filepath"
	runtimeDebug "runtime/debug"

	"github.com/pkg/errors"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
)

type SingBox struct {
	Running  bool
	ConfPath string
	instance *box.Box
	cancel   context.CancelFunc
}

func (s *SingBox) Close() {
	if s.instance != nil {
		_ = s.instance.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.Running = false
}

func (s *SingBox) Start(configPath string) error {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	instance, cancel, err := create(configPath)
	if err != nil {
		return err
	}
	runtimeDebug.FreeOSMemory()

	s.Running = true
	s.instance = instance
	s.cancel = cancel

	return nil
}

func create(configPath string) (*box.Box, context.CancelFunc, error) {
	options, err := readConfig(configPath)
	if err != nil {
		return nil, nil, err
	}

	if options.Log == nil {
		options.Log = &option.LogOptions{}
	}
	options.Log.DisableColor = true
	options.Log.Output = filepath.Join(ConfDir, "singbox.log")

	if options.Route.Geosite == nil {
		options.Route.Geosite = &option.GeositeOptions{}
	}
	if options.Route.Geosite.Path == "" {
		options.Route.Geosite.Path = filepath.Join(ConfDir, "geosite.db")
	}
	if options.Route.GeoIP == nil {
		options.Route.GeoIP = &option.GeoIPOptions{}
	}
	if options.Route.GeoIP.Path == "" {
		options.Route.GeoIP.Path = filepath.Join(ConfDir, "geoip.db")
	}

	ctx, cancel := context.WithCancel(context.Background())
	instance, err := box.New(ctx, options)
	if err != nil {
		cancel()
		return nil, nil, errors.Wrap(err, "create service")
	}
	err = instance.Start()
	if err != nil {
		cancel()
		return nil, nil, errors.Wrap(err, "start service")
	}
	return instance, cancel, nil
}

func readConfig(configPath string) (option.Options, error) {
	var (
		configContent []byte
		err           error
	)
	configContent, err = os.ReadFile(configPath)
	if err != nil {
		return option.Options{}, errors.Wrap(err, "read config")
	}

	var options option.Options
	err = options.UnmarshalJSON(jsonc.ToJSON(configContent))
	if err != nil {
		return option.Options{}, errors.Wrap(err, "decode config")
	}
	return options, nil
}
