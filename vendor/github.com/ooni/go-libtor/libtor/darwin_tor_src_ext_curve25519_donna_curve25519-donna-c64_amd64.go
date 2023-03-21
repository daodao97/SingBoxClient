// go-libtor - Self-contained Tor from Go
// Copyright (c) 2018 Péter Szilágyi. All rights reserved.
// +build darwin,amd64 darwin,arm64 ios,amd64 ios,arm64

package libtor

/*
#define BUILDDIR ""

#include <../src/ext/curve25519_donna/curve25519-donna-c64.c>
*/
import "C"
