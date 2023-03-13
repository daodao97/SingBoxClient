//go:build !unsafe_buffer && !disable_unsafe_buffer

package common

import "runtime"

// net/*Conn in windows keeps the buffer pointer passed in during io operations, so we disable it by default.
// https://github.com/golang/go/blob/4068be56ce7721a3d75606ea986d11e9ca27077a/src/internal/poll/fd_windows.go#L876

const UnsafeBuffer = runtime.GOOS != "windows"
