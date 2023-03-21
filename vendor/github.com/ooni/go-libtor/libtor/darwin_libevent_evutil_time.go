// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build darwin,amd64 darwin,arm64 ios,amd64 ios,arm64

package libtor

/*
#include <compat/sys/queue.h>
#include <../evutil_time.c>
*/
import "C"
