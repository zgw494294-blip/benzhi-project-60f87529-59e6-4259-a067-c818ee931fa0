package domain

import (
	"strings"
	"testing"
	"time"
)

func newTestAggregate(t *testing.T) (*Aggregate, time.Time) {
	t.Helper()
	now := time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)
	aggregate, err := CreateDataset(NewDataset{
		ID: "ds-1", Title: "测试声景", SiteCode: "SITE-1", CapturedFrom: now,
		CapturedTo: now.Add(time.Hour), TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"},
		DeviceCodes: []string{"REC-1"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return aggregate, now
}

func TestWorkflowWithRemediationAndRelease(t *testing.T) {
	aggregate, now := newTestAggregate(t)
	clip, err := aggregate.AddClip(NewClip{
		ID: "clip-1", SourceName: "forest.wav", StartedAt: now.Add(time.Minute), DurationMS: 10000,
		ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1",
		Metadata: map[string]string{"habitat": "森林"}, Now: now.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := aggregate.AddAnnotation(NewAnnotation{ID: "ann-1", ClipID: clip.ID, StartMS: 0, EndMS: 5000, LabelCode: "bird.a", Confidence: 0.4, CreatedBy: "标注员", Now: now.Add(2 * time.Second)})
	if err != nil || first.RevisionNo != 1 {
		t.Fatalf("首个标注失败: %#v %v", first, err)
	}
	issues, err := aggregate.SubmitForReview(now.Add(3*time.Second), func() string { return "issue-low" })
	if err != nil || len(issues) != 1 || aggregate.Dataset.Status != StatusRemediation {
		t.Fatalf("应生成阻断整改问题: %#v %v", issues, err)
	}
	if err := aggregate.Approve("复核员", now); !IsCode(err, "state_conflict") {
		t.Fatalf("存在阻断项时不应批准: %v", err)
	}
	second, err := aggregate.AddAnnotation(NewAnnotation{ID: "ann-2", ClipID: clip.ID, StartMS: 0, EndMS: 8000, LabelCode: "bird.a", Confidence: 0.92, CreatedBy: "标注员", Now: now.Add(4 * time.Second)})
	if err != nil || second.RevisionNo != 2 {
		t.Fatalf("整改修订失败: %#v %v", second, err)
	}
	if _, err := aggregate.ResolveIssue(issues[0].ID, second.ID, "复核员", now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.Approve("复核员", now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	manifest, err := aggregate.Freeze(now.Add(7 * time.Second))
	if err != nil || len(manifest.Clips) != 1 || len(manifest.Digest) != 64 {
		t.Fatalf("冻结清单无效: %#v %v", manifest, err)
	}
	credential, err := aggregate.Release("cred-1", 1, "负责人", now.Add(8*time.Second))
	if err != nil || credential.ManifestDigest != manifest.Digest || aggregate.Dataset.Status != StatusReleased {
		t.Fatalf("发布失败: %#v %v", credential, err)
	}
	if _, err := aggregate.AddAnnotation(NewAnnotation{}); !IsCode(err, "state_conflict") {
		t.Fatalf("发布后仍可编辑: %v", err)
	}
}

func TestClipRejectsDuplicateDigestAndOutOfRange(t *testing.T) {
	aggregate, now := newTestAggregate(t)
	base := NewClip{ID: "clip-1", SourceName: "one.wav", StartedAt: now, DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("b", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "林地"}, Now: now}
	if _, err := aggregate.AddClip(base); err != nil {
		t.Fatal(err)
	}
	base.ID, base.SourceName = "clip-2", "two.wav"
	if _, err := aggregate.AddClip(base); !IsCode(err, "state_conflict") {
		t.Fatalf("重复摘要未拒绝: %v", err)
	}
	base.ID, base.SHA256, base.StartedAt = "clip-3", strings.Repeat("c", 64), now.Add(2*time.Hour)
	if _, err := aggregate.AddClip(base); !IsCode(err, "validation_failed") {
		t.Fatalf("越界时间未拒绝: %v", err)
	}
}
