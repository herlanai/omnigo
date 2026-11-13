package watch

import (
	"context"
	"io/fs"
)

type identity struct {
	device, inode uint64
	kind          fs.FileMode
}
type identityCounts struct {
	before, after, created int
	destination            string
}

func identityIndex(ctx context.Context, before, after, created map[string]fs.FileInfo) (map[identity]*identityCounts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	index := make(map[identity]*identityCounts, len(before)+len(created))
	for stage, entries := range []map[string]fs.FileInfo{before, after, created} {
		for name, info := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key, ok := fileIdentity(info)
			if !ok {
				return nil, nil
			} // no partially indexed snapshot: unknown aliases may exist
			count := index[key]
			if count == nil {
				count = &identityCounts{}
				index[key] = count
			}
			switch stage {
			case 0:
				count.before++
			case 1:
				count.after++
			case 2:
				count.created++
				count.destination = name
			}
		}
	}
	return index, nil
}
func uniqueDestination(info fs.FileInfo, index map[identity]*identityCounts) string {
	key, ok := fileIdentity(info)
	if !ok {
		return ""
	}
	count := index[key]
	if count == nil || count.before != 1 || count.after != 1 || count.created != 1 {
		return ""
	}
	return count.destination
}
func compareDestination(info fs.FileInfo, before, after, created map[string]fs.FileInfo) string {
	countBefore, countAfter, countCreated := 0, 0, 0
	destination := ""
	for _, candidate := range before {
		if sameEntry(info, candidate) {
			countBefore++
		}
	}
	for _, candidate := range after {
		if sameEntry(info, candidate) {
			countAfter++
		}
	}
	for name, candidate := range created {
		if sameEntry(info, candidate) {
			countCreated++
			destination = name
		}
	}
	if countBefore != 1 || countAfter != 1 || countCreated != 1 {
		return ""
	}
	return destination
}
