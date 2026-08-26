package historyerrorpoison_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestHistoryRecoversAfterTransientEventLogFailure(t *testing.T) {
	directory := t.TempDir()
	store, err := repository.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	_, _, err = store.Mutate("ds-history", 0, "create-history", "dataset.created", func(_ *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		aggregate, createErr := domain.CreateDataset(domain.NewDataset{
			ID: "ds-history", Title: "历史恢复测试", SiteCode: "SITE-H",
			CapturedFrom: now, CapturedTo: now.Add(time.Hour), TaxonomyVersion: "v1",
			TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"R1"}, Now: now,
		})
		return aggregate, json.RawMessage(`{"version":1}`), createErr
	})
	if err != nil {
		t.Fatal(err)
	}

	eventPath := filepath.Join(directory, "events.log")
	backupPath := filepath.Join(directory, "events.saved")
	if err := os.Rename(eventPath, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(eventPath, 0o750); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(store)
	if _, err := service.History("ds-history"); err == nil {
		t.Fatal("事件日志失效时历史查询意外成功")
	}
	if err := os.Remove(eventPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backupPath, eventPath); err != nil {
		t.Fatal(err)
	}

	events, err := service.History("ds-history")
	if err != nil {
		t.Fatalf("事件日志恢复后仍返回先前错误: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "dataset.created" {
		t.Fatalf("恢复后的历史记录不正确: %#v", events)
	}
}
