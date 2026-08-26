package snapshot_rollback_chain_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestSnapshotFailureRetryKeepsEventChainRecoverable(t *testing.T) {
	directory := t.TempDir()
	store, err := repository.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	projectionPath := filepath.Join(directory, "projection.json")
	if err := os.Remove(projectionPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(projectionPath, 0o750); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	input := application.CreateDatasetInput{
		CommandMeta: application.CommandMeta{ExpectedVersion: 0, IdempotencyKey: "snapshot-retry-create"},
		Title:       "快照重试链", SiteCode: "SNAPSHOT-SITE",
		CapturedFrom: now, CapturedTo: now.Add(time.Hour),
		TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.snapshot"}, DeviceCodes: []string{"REC-SNAPSHOT"},
	}
	if _, err := service.CreateDataset(input); err == nil {
		t.Fatal("快照路径失效时提交应返回错误")
	}
	if err := os.Remove(projectionPath); err != nil {
		t.Fatal(err)
	}
	retried, err := service.CreateDataset(input)
	if err != nil {
		t.Fatalf("恢复快照路径后幂等重试失败: %v", err)
	}

	restored, err := repository.Open(directory)
	if err != nil {
		t.Fatalf("幂等重试后事件链无法恢复: %v", err)
	}
	aggregate, err := restored.Get(retried.DatasetID)
	if err != nil {
		t.Fatalf("恢复后数据集不可读取: %v", err)
	}
	if aggregate.Dataset.Title != "快照重试链" || aggregate.Dataset.Version != 1 {
		t.Fatalf("恢复后的数据集不一致: %#v", aggregate.Dataset)
	}
}
