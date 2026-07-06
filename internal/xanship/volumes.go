package xanship

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type VolumeEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type VolumeVerificationResult struct {
	SourceOnly   []string `json:"source_only,omitempty"`
	TargetOnly   []string `json:"target_only,omitempty"`
	SizeMismatch []string `json:"size_mismatch,omitempty"`
}

func (r VolumeVerificationResult) OK() bool {
	return len(r.SourceOnly) == 0 && len(r.TargetOnly) == 0 && len(r.SizeMismatch) == 0
}

func CompareVolumeEntries(source, target []VolumeEntry) VolumeVerificationResult {
	sm := entryMap(source)
	tm := entryMap(target)
	var result VolumeVerificationResult
	for path, s := range sm {
		t, ok := tm[path]
		if !ok {
			result.SourceOnly = append(result.SourceOnly, path)
			continue
		}
		if s.Size != t.Size {
			result.SizeMismatch = append(result.SizeMismatch, fmt.Sprintf("%s (%d != %d)", path, s.Size, t.Size))
		}
	}
	for path := range tm {
		if _, ok := sm[path]; !ok {
			result.TargetOnly = append(result.TargetOnly, path)
		}
	}
	sort.Strings(result.SourceOnly)
	sort.Strings(result.TargetOnly)
	sort.Strings(result.SizeMismatch)
	return result
}

func entryMap(entries []VolumeEntry) map[string]VolumeEntry {
	out := make(map[string]VolumeEntry, len(entries))
	for _, entry := range entries {
		out[entry.Path] = entry
	}
	return out
}

func parseVolumeEntries(output string) []VolumeEntry {
	var entries []VolumeEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		size, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		path := strings.Join(fields[1:], " ")
		path = strings.TrimPrefix(path, "./")
		entries = append(entries, VolumeEntry{Path: path, Size: size})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}
