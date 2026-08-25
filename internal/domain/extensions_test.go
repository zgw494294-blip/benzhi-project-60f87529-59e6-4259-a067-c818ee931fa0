package domain

import (
	"strings"
	"testing"
	"time"
)

func TestBatchClipsIsAtomicAndTouchesVersionOnce(t *testing.T) {
	aggregate, now := newTestAggregate(t)
	inputs := []NewClip{
		{ID: "clip-a", SourceName: "a.wav", StartedAt: now, DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}},
		{ID: "clip-b", SourceName: "b.wav", StartedAt: now.Add(time.Second), DurationMS: 2000, ChannelCount: 2, SHA256: strings.Repeat("b", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林缘"}},
	}
	clips, err := aggregate.AddClips(inputs, now.Add(time.Minute))
	if err != nil || len(clips) != 2 || aggregate.Dataset.Version != 2 {
		t.Fatalf("批量登记失败: clips=%#v version=%d err=%v", clips, aggregate.Dataset.Version, err)
	}
	before := aggregate.Clone()
	invalid := []NewClip{
		{ID: "clip-c", SourceName: "c.wav", StartedAt: now, DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("c", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}},
		{ID: "clip-d", SourceName: "d.wav", StartedAt: now.Add(2 * time.Hour), DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}},
	}
	_, err = aggregate.AddClips(invalid, now.Add(2*time.Minute))
	business, ok := err.(*Error)
	if !ok || len(business.Issues) == 0 || business.Issues[0].RecordNo != 2 {
		t.Fatalf("批量错误未定位: %#v", err)
	}
	if aggregate.Dataset.Version != before.Dataset.Version || len(aggregate.Clips) != len(before.Clips) {
		t.Fatalf("失败批次改变了聚合: before=%d/%d after=%d/%d", before.Dataset.Version, len(before.Clips), aggregate.Dataset.Version, len(aggregate.Clips))
	}
}

func TestInheritedRevisionAndComparisonBoundary(t *testing.T) {
	aggregate, now := newTestAggregate(t)
	clip, err := aggregate.AddClip(NewClip{ID: "clip-a", SourceName: "a.wav", StartedAt: now, DurationMS: 5000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	base, err := aggregate.AddAnnotation(NewAnnotation{ID: "ann-a", ClipID: clip.ID, StartMS: 10, EndMS: 1000, LabelCode: "bird.a", Confidence: .8, Note: "原备注", CreatedBy: "甲", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	note := "只改备注"
	next, err := aggregate.ReviseAnnotation("ann-b", clip.ID, base.ID, AnnotationOverrides{Note: &note}, now.Add(time.Second))
	if err != nil || next.RevisionNo != 2 || next.StartMS != base.StartMS || next.Note != note {
		t.Fatalf("继承修订失败: %#v %v", next, err)
	}
	if aggregate.Annotations[clip.ID][0].Note != "原备注" {
		t.Fatal("历史修订被覆盖")
	}
}

func TestPreflightReopenTrailAndManifestPreview(t *testing.T) {
	aggregate, now := newTestAggregate(t)
	clip, err := aggregate.AddClip(NewClip{ID: "clip-a", SourceName: "a.wav", StartedAt: now, DurationMS: 5000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	version := aggregate.Dataset.Version
	findings, err := aggregate.ReviewPreflight()
	if err != nil || len(findings) != 1 || findings[0].RuleCode != "COVERAGE_MISSING" || aggregate.Dataset.Version != version {
		t.Fatalf("预检结果或只读语义错误: %#v version=%d err=%v", findings, aggregate.Dataset.Version, err)
	}
	revision, err := aggregate.AddAnnotation(NewAnnotation{ID: "ann-a", ClipID: clip.ID, StartMS: 0, EndMS: 1000, LabelCode: "bird.a", Confidence: .9, CreatedBy: "甲", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = aggregate.SubmitForReview(now, func() string { return "automatic" }); err != nil {
		t.Fatal(err)
	}
	issue, err := aggregate.AddReviewIssue(NewIssue{ID: "manual", ClipID: clip.ID, RuleCode: "MANUAL", Severity: "blocking", Description: "人工确认", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = aggregate.ResolveIssue(issue.ID, revision.ID, "复核员", now); err != nil {
		t.Fatal(err)
	}
	if err = aggregate.Approve("负责人", now); err != nil {
		t.Fatal(err)
	}
	reopened, err := aggregate.ReopenIssue(issue.ID, "结论证据失效", "复核员乙", now.Add(time.Second))
	if err != nil || aggregate.Dataset.Status != StatusRemediation || reopened.Status != IssueOpen || len(reopened.DecisionTrail) != 2 {
		t.Fatalf("重开或轨迹失败: %#v status=%s err=%v", reopened, aggregate.Dataset.Status, err)
	}
	if _, err = aggregate.ResolveIssue(issue.ID, revision.ID, "复核员丙", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = aggregate.Approve("负责人", now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	first, err := aggregate.PreviewManifest()
	if err != nil {
		t.Fatal(err)
	}
	second, err := aggregate.PreviewManifest()
	if err != nil || first.Digest != second.Digest || first.BaseVersion != aggregate.Dataset.Version {
		t.Fatalf("清单预览不稳定: %#v %#v %v", first, second, err)
	}
	if _, err = aggregate.FreezeConfirmed(strings.Repeat("0", 64), now); !IsCode(err, "state_conflict") {
		t.Fatalf("错误摘要未拒绝: %v", err)
	}
	manifest, err := aggregate.FreezeConfirmed(first.Digest, now)
	if err != nil || manifest.Digest != first.Digest || len(manifest.Clips) != len(first.Clips) {
		t.Fatalf("确认冻结失败: %#v %v", manifest, err)
	}
}
