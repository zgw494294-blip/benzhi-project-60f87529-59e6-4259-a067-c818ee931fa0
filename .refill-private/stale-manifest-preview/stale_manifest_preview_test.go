package stalemanifestpreview_test

import (
	"strings"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestManifestPreviewRefreshesAfterRemediation(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)

	created, err := service.CreateDataset(application.CreateDatasetInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "stale-preview-create"},
		Title:       "缓存生命周期复现", SiteCode: "SITE-CACHE", CapturedFrom: now, CapturedTo: now.Add(time.Hour),
		TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"REC-1"},
	})
	mustMutation(t, created, err)
	registered, err := service.AddClip(created.DatasetID, application.AddClipInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: created.Version, IdempotencyKey: "stale-preview-clip"},
		SourceName:  "forest.wav", StartedAt: now.Add(time.Minute), DurationMS: 10_000, ChannelCount: 1,
		SHA256: strings.Repeat("a", 64), DeviceCode: "REC-1", Metadata: map[string]string{"habitat": "forest"},
	})
	mustMutation(t, registered, err)
	aggregate := mustDataset(t, service, created.DatasetID)
	clipID := onlyClipID(t, aggregate)

	annotated, err := service.AddAnnotation(created.DatasetID, application.AddAnnotationInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: registered.Version, IdempotencyKey: "stale-preview-low"},
		ClipID:      clipID, StartMS: 0, EndMS: 5_000, LabelCode: "bird.a", Confidence: 0.4,
		Note: "待整改", CreatedBy: "标注员",
	})
	mustMutation(t, annotated, err)
	submitted, err := service.SubmitReview(created.DatasetID, application.SubmitReviewInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: annotated.Version, IdempotencyKey: "stale-preview-submit"},
	})
	mustMutation(t, submitted, err)
	aggregate = mustDataset(t, service, created.DatasetID)
	issueID := onlyIssueID(t, aggregate)

	highConfidence := 0.92
	updatedNote := "首次整改"
	revised, err := service.ReviseAnnotation(created.DatasetID, application.ReviseAnnotationInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: submitted.Version, IdempotencyKey: "stale-preview-revise-one"},
		ClipID:      clipID, SourceRevisionID: latestRevisionID(t, aggregate, clipID), Confidence: &highConfidence, Note: &updatedNote,
	})
	mustMutation(t, revised, err)
	aggregate = mustDataset(t, service, created.DatasetID)
	resolved, err := service.ResolveIssue(created.DatasetID, issueID, application.ResolveIssueInput{
		CommandMeta:          application.CommandMeta{ExpectedVersion: revised.Version, IdempotencyKey: "stale-preview-resolve-one"},
		ResolutionRevisionID: latestRevisionID(t, aggregate, clipID), ReviewedBy: "复核员",
	})
	mustMutation(t, resolved, err)
	approved, err := service.Approve(created.DatasetID, application.ApproveInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: resolved.Version, IdempotencyKey: "stale-preview-approve-one"}, ReviewedBy: "复核员",
	})
	mustMutation(t, approved, err)
	firstPreview, err := service.PreviewManifest(created.DatasetID)
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := service.ReopenIssue(created.DatasetID, issueID, application.ReopenIssueInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: approved.Version, IdempotencyKey: "stale-preview-reopen"},
		Reason:      "需要补充鉴定依据", ReopenedBy: "复核员",
	})
	mustMutation(t, reopened, err)
	aggregate = mustDataset(t, service, created.DatasetID)
	secondNote := "补充声谱鉴定依据"
	revisedAgain, err := service.ReviseAnnotation(created.DatasetID, application.ReviseAnnotationInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: reopened.Version, IdempotencyKey: "stale-preview-revise-two"},
		ClipID:      clipID, SourceRevisionID: latestRevisionID(t, aggregate, clipID), Note: &secondNote,
	})
	mustMutation(t, revisedAgain, err)
	aggregate = mustDataset(t, service, created.DatasetID)
	resolvedAgain, err := service.ResolveIssue(created.DatasetID, issueID, application.ResolveIssueInput{
		CommandMeta:          application.CommandMeta{ExpectedVersion: revisedAgain.Version, IdempotencyKey: "stale-preview-resolve-two"},
		ResolutionRevisionID: latestRevisionID(t, aggregate, clipID), ReviewedBy: "复核员",
	})
	mustMutation(t, resolvedAgain, err)
	approvedAgain, err := service.Approve(created.DatasetID, application.ApproveInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: resolvedAgain.Version, IdempotencyKey: "stale-preview-approve-two"}, ReviewedBy: "复核员",
	})
	mustMutation(t, approvedAgain, err)

	secondPreview, err := service.PreviewManifest(created.DatasetID)
	if err != nil {
		t.Fatal(err)
	}
	if secondPreview.BaseVersion != approvedAgain.Version || secondPreview.Digest == firstPreview.Digest {
		t.Errorf("整改后预览未刷新: firstVersion=%d secondVersion=%d currentVersion=%d firstDigest=%s secondDigest=%s",
			firstPreview.BaseVersion, secondPreview.BaseVersion, approvedAgain.Version, firstPreview.Digest, secondPreview.Digest)
	}
	if _, err := service.Freeze(created.DatasetID, application.FreezeInput{
		CommandMeta:   application.CommandMeta{ExpectedVersion: approvedAgain.Version, IdempotencyKey: "stale-preview-freeze"},
		PreviewDigest: secondPreview.Digest,
	}); err != nil {
		t.Fatalf("刚返回的冻结预览摘要应可立即确认: %v", err)
	}
}

func mustMutation(t *testing.T, result application.MutationResult, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if result.DatasetID == "" || result.Version < 1 {
		t.Fatalf("无效变更结果: %#v", result)
	}
}

func mustDataset(t *testing.T, service *application.Service, datasetID string) *domain.Aggregate {
	t.Helper()
	aggregate, err := service.Dataset(datasetID)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func onlyClipID(t *testing.T, aggregate *domain.Aggregate) string {
	t.Helper()
	if len(aggregate.Clips) != 1 {
		t.Fatalf("期望一个片段，实际为 %d", len(aggregate.Clips))
	}
	for id := range aggregate.Clips {
		return id
	}
	return ""
}

func onlyIssueID(t *testing.T, aggregate *domain.Aggregate) string {
	t.Helper()
	if len(aggregate.Issues) != 1 {
		t.Fatalf("期望一个阻断问题，实际为 %d", len(aggregate.Issues))
	}
	for id, issue := range aggregate.Issues {
		if issue.Severity != "blocking" {
			t.Fatalf("期望阻断问题，实际为 %s", issue.Severity)
		}
		return id
	}
	return ""
}

func latestRevisionID(t *testing.T, aggregate *domain.Aggregate, clipID string) string {
	t.Helper()
	revisions := aggregate.Annotations[clipID]
	if len(revisions) == 0 {
		t.Fatal("片段缺少标注修订")
	}
	return revisions[len(revisions)-1].ID
}
