package domain

import (
	"sort"
	"strings"
	"time"
)

type NewDataset struct {
	ID              string
	Title           string
	SiteCode        string
	CapturedFrom    time.Time
	CapturedTo      time.Time
	TaxonomyVersion string
	TaxonomyCodes   []string
	DeviceCodes     []string
	Now             time.Time
}

type DatasetMetadata struct {
	Title           string
	SiteCode        string
	CapturedFrom    time.Time
	CapturedTo      time.Time
	TaxonomyVersion string
	TaxonomyCodes   []string
	DeviceCodes     []string
	Now             time.Time
}

func CreateDataset(input NewDataset) (*Aggregate, error) {
	if strings.TrimSpace(input.ID) == "" {
		return nil, Invalid("id", "数据集标识不能为空")
	}
	if err := ValidateDatasetInput(input.Title, input.SiteCode, input.TaxonomyVersion, input.CapturedFrom, input.CapturedTo, input.DeviceCodes); err != nil {
		return nil, err
	}
	codes, err := NormalizeCodes(input.TaxonomyCodes)
	if err != nil {
		return nil, err
	}
	devices := append([]string(nil), input.DeviceCodes...)
	sort.Strings(devices)
	now := input.Now.UTC()
	return &Aggregate{
		Dataset: Dataset{
			ID: input.ID, Title: strings.TrimSpace(input.Title), SiteCode: strings.TrimSpace(input.SiteCode),
			CapturedFrom: input.CapturedFrom.UTC(), CapturedTo: input.CapturedTo.UTC(),
			TaxonomyVersion: strings.TrimSpace(input.TaxonomyVersion), TaxonomyCodes: codes,
			DeviceCodes: devices, Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		Clips: make(map[string]AudioClip), Annotations: make(map[string][]AnnotationRevision), Issues: make(map[string]ReviewIssue),
	}, nil
}

func (a *Aggregate) Clone() *Aggregate {
	if a == nil {
		return nil
	}
	copyValue := *a
	copyValue.Dataset.TaxonomyCodes = append([]string(nil), a.Dataset.TaxonomyCodes...)
	copyValue.Dataset.DeviceCodes = append([]string(nil), a.Dataset.DeviceCodes...)
	copyValue.Clips = make(map[string]AudioClip, len(a.Clips))
	for id, clip := range a.Clips {
		clip.Metadata = cloneStringMap(clip.Metadata)
		copyValue.Clips[id] = clip
	}
	copyValue.Annotations = make(map[string][]AnnotationRevision, len(a.Annotations))
	for clipID, revisions := range a.Annotations {
		copyValue.Annotations[clipID] = append([]AnnotationRevision(nil), revisions...)
	}
	copyValue.Issues = make(map[string]ReviewIssue, len(a.Issues))
	for issueID, issue := range a.Issues {
		issue.DecisionTrail = append([]IssueDecision(nil), issue.DecisionTrail...)
		copyValue.Issues[issueID] = issue
	}
	if a.Manifest != nil {
		manifest := *a.Manifest
		manifest.Clips = append([]ManifestClip(nil), a.Manifest.Clips...)
		for i := range manifest.Clips {
			manifest.Clips[i].Annotations = append([]AnnotationRevision(nil), manifest.Clips[i].Annotations...)
		}
		copyValue.Manifest = &manifest
	}
	if a.Credential != nil {
		credential := *a.Credential
		copyValue.Credential = &credential
	}
	return &copyValue
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (a *Aggregate) touch(now time.Time) {
	a.Dataset.Version++
	a.Dataset.UpdatedAt = now.UTC()
}

func (a *Aggregate) UpdateMetadata(input DatasetMetadata) error {
	if a.Dataset.Status != StatusDraft {
		return Conflict("只有 draft 数据集可维护采集范围与分类体系")
	}
	if err := ValidateDatasetInput(input.Title, input.SiteCode, input.TaxonomyVersion, input.CapturedFrom, input.CapturedTo, input.DeviceCodes); err != nil {
		return err
	}
	codes, err := NormalizeCodes(input.TaxonomyCodes)
	if err != nil {
		return err
	}
	for _, clip := range a.Clips {
		end := clip.StartedAt.Add(time.Duration(clip.DurationMS) * time.Millisecond)
		if clip.StartedAt.Before(input.CapturedFrom) || end.After(input.CapturedTo) {
			return Conflict("新的采集时间窗不能排除已登记片段")
		}
		if !contains(input.DeviceCodes, clip.DeviceCode) {
			return Conflict("新的设备清单不能排除已登记片段所用设备")
		}
		for _, revision := range a.Annotations[clip.ID] {
			if !contains(codes, revision.LabelCode) {
				return Conflict("新的分类编码不能排除已提交标注使用的编码")
			}
		}
	}
	a.Dataset.Title = strings.TrimSpace(input.Title)
	a.Dataset.SiteCode = strings.TrimSpace(input.SiteCode)
	a.Dataset.CapturedFrom = input.CapturedFrom.UTC()
	a.Dataset.CapturedTo = input.CapturedTo.UTC()
	a.Dataset.TaxonomyVersion = strings.TrimSpace(input.TaxonomyVersion)
	a.Dataset.TaxonomyCodes = codes
	a.Dataset.DeviceCodes = append([]string(nil), input.DeviceCodes...)
	sort.Strings(a.Dataset.DeviceCodes)
	a.touch(input.Now)
	return nil
}

func (a *Aggregate) editableForCatalog() error {
	if a.Dataset.Status != StatusDraft {
		return Conflict("只有 draft 数据集可修改片段目录")
	}
	return nil
}

func (a *Aggregate) editableForAnnotation() error {
	if a.Dataset.Status != StatusDraft && a.Dataset.Status != StatusRemediation {
		return Conflict("只有 draft 或 remediation 数据集可提交标注修订")
	}
	return nil
}
