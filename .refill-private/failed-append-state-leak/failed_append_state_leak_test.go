package failed_append_state_leak_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestFailedEventAppendDoesNotLeakAggregateChildren(t *testing.T) {
	storageDir := t.TempDir()
	store, err := repository.Open(storageDir)
	if err != nil {
		t.Fatalf("打开存储: %v", err)
	}
	service := application.NewService(store)
	from := time.Date(2026, time.August, 25, 1, 0, 0, 0, time.UTC)

	created, err := service.CreateDataset(application.CreateDatasetInput{
		CommandMeta:     application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-before-log-failure"},
		Title:           "日志失效隔离测试",
		SiteCode:        "site-a",
		CapturedFrom:    from,
		CapturedTo:      from.Add(time.Hour),
		TaxonomyVersion: "birds-v1",
		TaxonomyCodes:   []string{"bird.call"},
		DeviceCodes:     []string{"recorder-a"},
	})
	if err != nil {
		t.Fatalf("创建数据集: %v", err)
	}
	datasetID := created.DatasetID
	if _, err := service.AddClip(datasetID, application.AddClipInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 1, IdempotencyKey: "clip-before-log-failure"},
		SourceName:  "forest.wav", StartedAt: from.Add(time.Minute), DurationMS: 10_000,
		ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "recorder-a",
		Metadata: map[string]string{"habitat": "forest"},
	}); err != nil {
		t.Fatalf("登记片段: %v", err)
	}
	beforeAnnotation, err := service.Dataset(datasetID)
	if err != nil {
		t.Fatalf("读取标注失败前投影: %v", err)
	}
	clipID := ""
	for id := range beforeAnnotation.Clips {
		clipID = id
	}
	if clipID == "" {
		t.Fatal("未找到已登记片段")
	}
	committedAnnotationCount := len(beforeAnnotation.Annotations[clipID])

	eventPath := filepath.Join(storageDir, "events.log")
	firstCommittedPath := filepath.Join(storageDir, "events.before-annotation")
	if err := os.Rename(eventPath, firstCommittedPath); err != nil {
		t.Fatalf("暂存标注前事件日志: %v", err)
	}
	if err := os.Mkdir(eventPath, 0o750); err != nil {
		t.Fatalf("制造标注提交时的日志失效: %v", err)
	}
	startMS, endMS := int64(0), int64(1_000)
	labelCode, confidence, createdBy := "bird.call", 0.9, "annotator-a"
	_, err = service.ReviseAnnotation(datasetID, application.ReviseAnnotationInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "annotation-during-log-failure"},
		ClipID:      clipID, StartMS: &startMS, EndMS: &endMS, LabelCode: &labelCode,
		Confidence: &confidence, CreatedBy: &createdBy,
	})
	if err == nil {
		t.Fatal("事件日志失效时提交标注应返回错误")
	}
	afterAnnotation, err := service.Dataset(datasetID)
	if err != nil {
		t.Fatalf("读取标注失败后投影: %v", err)
	}
	if len(afterAnnotation.Annotations[clipID]) != committedAnnotationCount {
		t.Errorf("提交失败泄漏了未持久化标注: before=%d after=%d", committedAnnotationCount, len(afterAnnotation.Annotations[clipID]))
	}
	if err := os.Remove(eventPath); err != nil {
		t.Fatalf("移除第一次失效日志路径: %v", err)
	}
	if err := os.Rename(firstCommittedPath, eventPath); err != nil {
		t.Fatalf("恢复标注前事件日志: %v", err)
	}

	reopened, err := repository.Open(storageDir)
	if err != nil {
		t.Fatalf("标注失败后重新打开存储: %v", err)
	}
	recoveredAnnotation, err := reopened.Get(datasetID)
	if err != nil {
		t.Fatalf("读取标注恢复投影: %v", err)
	}
	if len(recoveredAnnotation.Annotations[clipID]) != 0 || recoveredAnnotation.Dataset.Version != 2 {
		t.Fatalf("标注失败后重启未回到最后提交状态: annotations=%d version=%d", len(recoveredAnnotation.Annotations[clipID]), recoveredAnnotation.Dataset.Version)
	}

	service = application.NewService(reopened)
	if _, err := service.SubmitReview(datasetID, application.SubmitReviewInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 2, IdempotencyKey: "submit-before-second-log-failure"},
	}); err != nil {
		t.Fatalf("提交复核: %v", err)
	}
	beforeIssue, err := service.Dataset(datasetID)
	if err != nil {
		t.Fatalf("读取问题失败前投影: %v", err)
	}
	committedIssueCount := len(beforeIssue.Issues)
	secondCommittedPath := filepath.Join(storageDir, "events.before-issue")
	if err := os.Rename(eventPath, secondCommittedPath); err != nil {
		t.Fatalf("暂存问题登记前事件日志: %v", err)
	}
	if err := os.Mkdir(eventPath, 0o750); err != nil {
		t.Fatalf("制造问题提交时的日志失效: %v", err)
	}
	_, err = service.AddIssue(datasetID, application.AddIssueInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 3, IdempotencyKey: "issue-during-log-failure"},
		RuleCode:    "manual-noise", Severity: "advisory", Description: "该问题不得在提交失败后留在内存投影中",
	})
	if err == nil {
		t.Fatal("事件日志失效时登记问题应返回错误")
	}
	afterIssue, err := service.Dataset(datasetID)
	if err != nil {
		t.Fatalf("读取问题失败后投影: %v", err)
	}
	if len(afterIssue.Issues) != committedIssueCount {
		t.Errorf("提交失败泄漏了未持久化问题: before=%d after=%d", committedIssueCount, len(afterIssue.Issues))
	}
	if afterIssue.Dataset.Version != beforeIssue.Dataset.Version || afterIssue.Dataset.Status != beforeIssue.Dataset.Status {
		t.Errorf("提交失败改变了已提交状态: before=%d/%s after=%d/%s", beforeIssue.Dataset.Version, beforeIssue.Dataset.Status, afterIssue.Dataset.Version, afterIssue.Dataset.Status)
	}
	if err := os.Remove(eventPath); err != nil {
		t.Fatalf("移除第二次失效日志路径: %v", err)
	}
	if err := os.Rename(secondCommittedPath, eventPath); err != nil {
		t.Fatalf("恢复问题登记前事件日志: %v", err)
	}
	finalStore, err := repository.Open(storageDir)
	if err != nil {
		t.Fatalf("问题失败后重新打开存储: %v", err)
	}
	recoveredIssue, err := finalStore.Get(datasetID)
	if err != nil {
		t.Fatalf("读取问题恢复投影: %v", err)
	}
	if len(recoveredIssue.Issues) != committedIssueCount || recoveredIssue.Dataset.Version != beforeIssue.Dataset.Version || recoveredIssue.Dataset.Status != beforeIssue.Dataset.Status {
		t.Fatalf("问题失败后重启未回到最后提交状态: issues=%d version=%d status=%s", len(recoveredIssue.Issues), recoveredIssue.Dataset.Version, recoveredIssue.Dataset.Status)
	}
}
