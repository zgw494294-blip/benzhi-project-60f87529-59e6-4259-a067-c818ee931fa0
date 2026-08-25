package domain

import (
	"strings"
	"time"
)

type NewClip struct {
	ID           string
	SourceName   string
	StartedAt    time.Time
	DurationMS   int64
	ChannelCount int
	SHA256       string
	DeviceCode   string
	Metadata     map[string]string
	Now          time.Time
}

func (a *Aggregate) AddClip(input NewClip) (AudioClip, error) {
	if err := a.editableForCatalog(); err != nil {
		return AudioClip{}, err
	}
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.SourceName) == "" {
		return AudioClip{}, Invalid("sourceName", "片段标识和来源名称不能为空")
	}
	if _, exists := a.Clips[input.ID]; exists {
		return AudioClip{}, Conflict("片段标识已存在")
	}
	if input.DurationMS <= 0 || input.DurationMS > 24*60*60*1000 {
		return AudioClip{}, Invalid("durationMs", "片段时长必须在 1 毫秒到 24 小时之间")
	}
	if input.ChannelCount < 1 || input.ChannelCount > 64 {
		return AudioClip{}, Invalid("channelCount", "声道数必须在 1 到 64 之间")
	}
	if !validSHA256(strings.ToLower(input.SHA256)) {
		return AudioClip{}, Invalid("sha256", "校验摘要必须是 64 位十六进制 SHA-256")
	}
	if !contains(a.Dataset.DeviceCodes, input.DeviceCode) {
		return AudioClip{}, Invalid("deviceCode", "片段设备未在数据集中登记")
	}
	end := input.StartedAt.Add(time.Duration(input.DurationMS) * time.Millisecond)
	if input.StartedAt.Before(a.Dataset.CapturedFrom) || end.After(a.Dataset.CapturedTo) {
		return AudioClip{}, Invalid("startedAt", "片段时间范围超出数据集采集时间窗")
	}
	for _, clip := range a.Clips {
		if strings.EqualFold(clip.SHA256, input.SHA256) {
			return AudioClip{}, Conflict("校验摘要已被其他片段使用")
		}
	}
	if len(input.Metadata) == 0 || strings.TrimSpace(input.Metadata["habitat"]) == "" {
		return AudioClip{}, Invalid("metadata", "采集元数据必须包含 habitat")
	}
	clip := AudioClip{
		ID: input.ID, DatasetID: a.Dataset.ID, SourceName: strings.TrimSpace(input.SourceName),
		StartedAt: input.StartedAt.UTC(), DurationMS: input.DurationMS, ChannelCount: input.ChannelCount,
		SHA256: strings.ToLower(input.SHA256), DeviceCode: input.DeviceCode, Metadata: cloneStringMap(input.Metadata),
	}
	a.Clips[clip.ID] = clip
	a.touch(input.Now)
	return clip, nil
}

// AddClips 先在聚合副本上逐条应用与单条登记完全相同的规则。只有全部记录
// 合法时才替换片段目录，并把整个批次视为一次业务变更。
func (a *Aggregate) AddClips(inputs []NewClip, now time.Time) ([]AudioClip, error) {
	if err := a.editableForCatalog(); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, Invalid("clips", "批量登记至少包含一条片段记录")
	}
	if len(inputs) > 500 {
		return nil, Invalid("clips", "单次批量登记不得超过 500 条片段记录")
	}
	working := a.Clone()
	baseVersion := a.Dataset.Version
	registered := make([]AudioClip, 0, len(inputs))
	issues := make([]ValidationIssue, 0)
	seenDigests := make(map[string]bool, len(a.Clips)+len(inputs))
	for _, clip := range a.Clips {
		seenDigests[strings.ToLower(clip.SHA256)] = true
	}
	for index, input := range inputs {
		input.Now = now
		recordIssues := validateBatchClip(a, input, index)
		digest := strings.ToLower(input.SHA256)
		if validSHA256(digest) {
			if seenDigests[digest] {
				recordIssues = append(recordIssues, ValidationIssue{RecordIndex: index, RecordNo: index + 1, Field: "sha256", Code: "duplicate_digest", Message: "校验摘要已被数据集或本批次中的其他片段使用"})
			} else {
				seenDigests[digest] = true
			}
		}
		if len(recordIssues) > 0 {
			issues = append(issues, recordIssues...)
			continue
		}
		clip, err := working.AddClip(input)
		if err != nil {
			field, code, message := clipErrorDetails(err)
			issues = append(issues, ValidationIssue{RecordIndex: index, RecordNo: index + 1, Field: field, Code: code, Message: message})
			continue
		}
		registered = append(registered, clip)
	}
	if len(issues) > 0 {
		return nil, InvalidBatch(issues)
	}
	working.Dataset.Version = baseVersion + 1
	working.Dataset.UpdatedAt = now.UTC()
	*a = *working
	return registered, nil
}

func validateBatchClip(a *Aggregate, input NewClip, index int) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(field, code, message string) {
		issues = append(issues, ValidationIssue{RecordIndex: index, RecordNo: index + 1, Field: field, Code: code, Message: message})
	}
	if strings.TrimSpace(input.SourceName) == "" {
		add("sourceName", "required", "来源名称不能为空")
	}
	if input.DurationMS <= 0 || input.DurationMS > 24*60*60*1000 {
		add("durationMs", "duration_out_of_range", "片段时长必须在 1 毫秒到 24 小时之间")
	}
	if input.ChannelCount < 1 || input.ChannelCount > 64 {
		add("channelCount", "channel_out_of_range", "声道数必须在 1 到 64 之间")
	}
	if !validSHA256(strings.ToLower(input.SHA256)) {
		add("sha256", "invalid_digest", "校验摘要必须是 64 位十六进制 SHA-256")
	}
	if !contains(a.Dataset.DeviceCodes, input.DeviceCode) {
		add("deviceCode", "unknown_device", "片段设备未在数据集中登记")
	}
	if input.StartedAt.IsZero() || input.StartedAt.Before(a.Dataset.CapturedFrom) || (input.DurationMS > 0 && input.StartedAt.Add(time.Duration(input.DurationMS)*time.Millisecond).After(a.Dataset.CapturedTo)) {
		add("startedAt", "time_out_of_range", "片段时间范围超出数据集采集时间窗")
	}
	if len(input.Metadata) == 0 || strings.TrimSpace(input.Metadata["habitat"]) == "" {
		add("metadata.habitat", "missing_habitat", "采集元数据必须包含 habitat")
	}
	return issues
}

func clipErrorDetails(err error) (string, string, string) {
	if business, ok := err.(*Error); ok {
		field := business.Field
		if field == "" {
			field = "sha256"
		}
		code := business.Code
		if business.Code == "state_conflict" && strings.Contains(business.Message, "摘要") {
			code = "duplicate_digest"
		}
		return field, code, business.Message
	}
	return "record", "validation_failed", err.Error()
}
