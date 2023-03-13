package tun

import E "github.com/sagernet/sing/common/exceptions"

type PackageManager interface {
	Start() error
	Close() error
	IDByPackage(packageName string) (uint32, bool)
	IDBySharedPackage(sharedPackage string) (uint32, bool)
	PackageByID(id uint32) (string, bool)
	SharedPackageByID(id uint32) (string, bool)
}

type PackageManagerCallback interface {
	OnPackagesUpdated(packages int, sharedUsers int)
	E.Handler
}
