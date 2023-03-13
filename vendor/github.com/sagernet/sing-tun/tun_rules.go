package tun

import (
	"context"
	"os"
	"sort"
	"strconv"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ranges"
)

const (
	androidUserRange        = 100000
	userEnd          uint32 = 0xFFFFFFFF - 1
)

func (o *Options) BuildAndroidRules(packageManager PackageManager, errorHandler E.Handler) {
	var includeUser []uint32
	if len(o.IncludeAndroidUser) > 0 {
		o.IncludeAndroidUser = common.Uniq(o.IncludeAndroidUser)
		sort.Ints(o.IncludeAndroidUser)
		var userExcludeRange []ranges.Range[uint32]
		for _, androidUser := range o.IncludeAndroidUser {
			includeUser = append(includeUser, uint32(androidUser))
			userExcludeRange = append(userExcludeRange, ranges.New[uint32](uint32(androidUser)*androidUserRange, uint32(androidUser+1)*androidUserRange-1))
		}
		userExcludeRange = ranges.Revert(0, userEnd, userExcludeRange)
		o.ExcludeUID = append(o.ExcludeUID, userExcludeRange...)
	}
	if len(includeUser) == 0 {
		userDirs, err := os.ReadDir("/data/user")
		if err == nil {
			var userId uint64
			for _, userDir := range userDirs {
				userId, err = strconv.ParseUint(userDir.Name(), 10, 32)
				if err != nil {
					continue
				}
				includeUser = append(includeUser, uint32(userId))
			}
		}
	}
	if len(includeUser) == 0 {
		includeUser = []uint32{0}
	}
	if len(o.IncludePackage) > 0 {
		o.IncludePackage = common.Uniq(o.IncludePackage)
		for _, packageName := range o.IncludePackage {
			if sharedId, loaded := packageManager.IDBySharedPackage(packageName); loaded {
				for _, androidUser := range includeUser {
					o.IncludeUID = append(o.IncludeUID, ranges.NewSingle(sharedId+androidUser*androidUserRange))
				}
				continue
			}
			if userId, loaded := packageManager.IDByPackage(packageName); loaded {
				for _, androidUser := range includeUser {
					o.IncludeUID = append(o.IncludeUID, ranges.NewSingle(userId+androidUser*androidUserRange))
				}
				continue
			}
			errorHandler.NewError(context.Background(), E.New("package to include not found: ", packageName))
		}
	}
	if len(o.ExcludePackage) > 0 {
		o.ExcludePackage = common.Uniq(o.ExcludePackage)
		for _, packageName := range o.ExcludePackage {
			if sharedId, loaded := packageManager.IDBySharedPackage(packageName); loaded {
				for _, androidUser := range includeUser {
					o.ExcludeUID = append(o.ExcludeUID, ranges.NewSingle(sharedId+androidUser*androidUserRange))
				}
			}
			if userId, loaded := packageManager.IDByPackage(packageName); loaded {
				for _, androidUser := range includeUser {
					o.ExcludeUID = append(o.ExcludeUID, ranges.NewSingle(userId+androidUser*androidUserRange))
				}
				continue
			}
			errorHandler.NewError(context.Background(), E.New("package to exclude not found: ", packageName))
		}
	}
}

func (o *Options) ExcludedRanges() (uidRanges []ranges.Range[uint32]) {
	return buildExcludedRanges(o.IncludeUID, o.ExcludeUID)
}

func buildExcludedRanges(includeRanges []ranges.Range[uint32], excludeRanges []ranges.Range[uint32]) (uidRanges []ranges.Range[uint32]) {
	uidRanges = includeRanges
	if len(uidRanges) > 0 {
		uidRanges = ranges.Exclude(uidRanges, excludeRanges)
		uidRanges = ranges.Revert(0, userEnd, uidRanges)
	} else {
		uidRanges = excludeRanges
	}
	return ranges.Merge(uidRanges)
}
