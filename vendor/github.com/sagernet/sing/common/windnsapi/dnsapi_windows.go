package windnsapi

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	moddnsapi                 = windows.NewLazySystemDLL("dnsapi.dll")
	procDnsFlushResolverCache = moddnsapi.NewProc("DnsFlushResolverCache")
)

func FlushResolverCache() error {
	r0, _, err := syscall.SyscallN(procDnsFlushResolverCache.Addr())
	if r0 == 0 {
		return os.NewSyscallError("DnsFlushResolverCache", err)
	}
	return nil
}
