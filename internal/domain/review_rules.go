package domain

import (
	"fmt"
	"sort"
	"strings"
)

type ReviewFinding struct {
	ClipID      string `json:"clipId,omitempty"`
	SourceName  string `json:"sourceName,omitempty"`
	RuleCode    string `json:"ruleCode"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

func (a *Aggregate) validateForReview() []ReviewFinding {
	findings := make([]ReviewFinding, 0)
	clipIDs := make([]string, 0, len(a.Clips))
	for id := range a.Clips {
		clipIDs = append(clipIDs, id)
	}
	sort.Strings(clipIDs)
	sources := make(map[string]string)
	labelConfidence := make(map[string][]struct {
		clipID     string
		confidence float64
	})
	for _, clipID := range clipIDs {
		clip := a.Clips[clipID]
		normalizedSource := strings.ToLower(strings.TrimSpace(clip.SourceName))
		if earlier, duplicate := sources[normalizedSource]; duplicate {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "SOURCE_NAME_DUPLICATE", Severity: "blocking", Description: fmt.Sprintf("来源名称与片段 %s 重复", earlier)})
		} else {
			sources[normalizedSource] = clipID
		}
		if strings.TrimSpace(clip.Metadata["habitat"]) == "" {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "METADATA_INCOMPLETE", Severity: "blocking", Description: "片段缺少 habitat 采集元数据"})
		}
		revisions := a.Annotations[clipID]
		if len(revisions) == 0 {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "COVERAGE_MISSING", Severity: "blocking", Description: "片段没有任何标注修订"})
			continue
		}
		latest := revisions[len(revisions)-1]
		if latest.StartMS < 0 || latest.EndMS <= latest.StartMS || latest.EndMS > clip.DurationMS {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "INTERVAL_OUT_OF_RANGE", Severity: "blocking", Description: "最新标注的覆盖区间超出片段范围"})
		}
		if !contains(a.Dataset.TaxonomyCodes, latest.LabelCode) {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "TAXONOMY_UNKNOWN", Severity: "blocking", Description: "最新标注使用了分类体系外编码"})
		}
		if latest.Confidence < 0 || latest.Confidence > 1 {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "CONFIDENCE_INVALID", Severity: "blocking", Description: "最新标注置信度超出 0 到 1"})
		} else if latest.Confidence < 0.5 {
			findings = append(findings, ReviewFinding{ClipID: clipID, SourceName: clip.SourceName, RuleCode: "LOW_CONFIDENCE", Severity: "blocking", Description: "最新标注置信度低于 0.5"})
		}
		labelConfidence[latest.LabelCode] = append(labelConfidence[latest.LabelCode], struct {
			clipID     string
			confidence float64
		}{clipID: clipID, confidence: latest.Confidence})
	}
	labels := make([]string, 0, len(labelConfidence))
	for label := range labelConfidence {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		values := labelConfidence[label]
		if len(values) < 2 {
			continue
		}
		minimum, maximum := values[0], values[0]
		for _, value := range values[1:] {
			if value.confidence < minimum.confidence {
				minimum = value
			}
			if value.confidence > maximum.confidence {
				maximum = value
			}
		}
		if maximum.confidence-minimum.confidence > 0.4 {
			findings = append(findings, ReviewFinding{ClipID: minimum.clipID, SourceName: a.Clips[minimum.clipID].SourceName, RuleCode: "CROSS_CLIP_CONFIDENCE_DRIFT", Severity: "advisory", Description: fmt.Sprintf("分类 %s 的跨片段置信度差异超过 0.4", label)})
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].ClipID == findings[j].ClipID {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return findings[i].ClipID < findings[j].ClipID
	})
	return findings
}

func (a *Aggregate) ReviewPreflight() ([]ReviewFinding, error) {
	if a.Dataset.Status != StatusDraft {
		return nil, Conflict("只有 draft 数据集可执行送审预检")
	}
	if len(a.Clips) == 0 {
		return nil, Conflict("至少登记一个片段后才能执行送审预检")
	}
	return a.validateForReview(), nil
}
