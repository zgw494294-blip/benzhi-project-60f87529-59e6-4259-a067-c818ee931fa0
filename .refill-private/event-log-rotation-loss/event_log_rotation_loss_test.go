package event_log_rotation_loss_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestRotatedEventLogDoesNotLoseAcknowledgedMutation(t *testing.T) {
	directory := t.TempDir()
	store, err := repository.Open(directory)
	if err != nil {
		t.Fatalf("打开存储: %v", err)
	}

	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	create := func(_ *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		aggregate, createErr := domain.CreateDataset(domain.NewDataset{
			ID: "ds-rotation", Title: "轮转前", SiteCode: "site-a",
			CapturedFrom: from, CapturedTo: to, TaxonomyVersion: "tax-v1",
			TaxonomyCodes: []string{"bird"}, DeviceCodes: []string{"recorder-a"}, Now: from,
		})
		return aggregate, json.RawMessage(`{"version":1}`), createErr
	}
	if _, _, err := store.Mutate("ds-rotation", 0, "create-rotation", "dataset.created", create); err != nil {
		t.Fatalf("创建数据集: %v", err)
	}

	eventPath := filepath.Join(directory, "events.log")
	if err := os.Rename(eventPath, filepath.Join(directory, "events.log.1")); err != nil {
		t.Fatalf("轮转事件日志: %v", err)
	}
	if err := os.WriteFile(eventPath, nil, 0o640); err != nil {
		t.Fatalf("重建事件日志路径: %v", err)
	}

	update := func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		err := current.UpdateMetadata(domain.DatasetMetadata{
			Title: "轮转后已确认", SiteCode: "site-a", CapturedFrom: from, CapturedTo: to,
			TaxonomyVersion: "tax-v1", TaxonomyCodes: []string{"bird"},
			DeviceCodes: []string{"recorder-a"}, Now: from.Add(time.Hour),
		})
		return current, json.RawMessage(`{"version":2}`), err
	}
	if _, _, err := store.Mutate("ds-rotation", 1, "update-after-rotation", "dataset.metadata_updated", update); err != nil {
		t.Fatalf("轮转后提交应答失败: %v", err)
	}

	reopened, err := repository.Open(directory)
	if err != nil {
		t.Fatalf("重启恢复: %v", err)
	}
	aggregate, err := reopened.Get("ds-rotation")
	if err != nil {
		t.Fatalf("已确认的数据集在重启后丢失: %v", err)
	}
	if aggregate.Dataset.Version != 2 || aggregate.Dataset.Title != "轮转后已确认" {
		t.Fatalf("重启后未恢复已确认更新: version=%d title=%q", aggregate.Dataset.Version, aggregate.Dataset.Title)
	}
}
