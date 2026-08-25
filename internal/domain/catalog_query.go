package domain

import (
	"sort"
	"strings"
	"time"
)

type ClipFilter struct {
	SourceName  string
	DeviceCode  string
	StartedFrom *time.Time
	StartedTo   *time.Time
	Habitat     string
}

func (a *Aggregate) NormalizeClipFilter(input ClipFilter) (ClipFilter, error) {
	input.SourceName = strings.TrimSpace(input.SourceName)
	input.DeviceCode = strings.TrimSpace(input.DeviceCode)
	input.Habitat = strings.TrimSpace(input.Habitat)
	if input.DeviceCode != "" && !contains(a.Dataset.DeviceCodes, input.DeviceCode) {
		return ClipFilter{}, Invalid("deviceCode", "筛选设备未在当前数据集中登记")
	}
	if input.StartedFrom != nil {
		value := input.StartedFrom.UTC()
		input.StartedFrom = &value
	}
	if input.StartedTo != nil {
		value := input.StartedTo.UTC()
		input.StartedTo = &value
	}
	if input.StartedFrom != nil && input.StartedTo != nil && input.StartedTo.Before(*input.StartedFrom) {
		return ClipFilter{}, Invalid("startedTo", "采集结束筛选时间不得早于开始时间")
	}
	return input, nil
}

func (a *Aggregate) FilterClips(filter ClipFilter) ([]AudioClip, error) {
	filter, err := a.NormalizeClipFilter(filter)
	if err != nil {
		return nil, err
	}
	clips := make([]AudioClip, 0, len(a.Clips))
	for _, clip := range a.Clips {
		if filter.SourceName != "" && !strings.Contains(strings.ToLower(clip.SourceName), strings.ToLower(filter.SourceName)) {
			continue
		}
		if filter.DeviceCode != "" && clip.DeviceCode != filter.DeviceCode {
			continue
		}
		if filter.StartedFrom != nil && clip.StartedAt.Before(*filter.StartedFrom) {
			continue
		}
		if filter.StartedTo != nil && clip.StartedAt.After(*filter.StartedTo) {
			continue
		}
		if filter.Habitat != "" && !strings.EqualFold(strings.TrimSpace(clip.Metadata["habitat"]), filter.Habitat) {
			continue
		}
		clip.Metadata = cloneStringMap(clip.Metadata)
		clips = append(clips, clip)
	}
	sort.SliceStable(clips, func(i, j int) bool {
		if !clips[i].StartedAt.Equal(clips[j].StartedAt) {
			return clips[i].StartedAt.Before(clips[j].StartedAt)
		}
		if clips[i].SourceName != clips[j].SourceName {
			return clips[i].SourceName < clips[j].SourceName
		}
		return clips[i].ID < clips[j].ID
	})
	return clips, nil
}
