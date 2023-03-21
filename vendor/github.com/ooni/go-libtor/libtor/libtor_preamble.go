package libtor

/*
#cgo linux,amd64,!android linux,arm64,!android CFLAGS: -DARCH_LINUX64
#cgo linux,386,!android linux,arm,!android     CFLAGS: -DARCH_LINUX32
#cgo darwin,amd64,!ios darwin,arm64,!ios       CFLAGS: -DARCH_MACOS64
#cgo ios,amd64 ios,arm64                       CFLAGS: -DARCH_IOS64
#cgo android,amd64 android,arm64               CFLAGS: -DARCH_ANDROID64
#cgo android,386 android,arm                   CFLAGS: -DARCH_ANDROID32
*/
import "C"
