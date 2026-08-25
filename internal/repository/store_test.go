package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func TestStoreRestoresFromEventLogAndReplaysIdempotency(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mutation := func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		aggregate, err := domain.CreateDataset(domain.NewDataset{ID: "ds-restore", Title: "恢复测试", SiteCode: "S", CapturedFrom: now, CapturedTo: now.Add(time.Hour), TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"R1"}, Now: now})
		return aggregate, json.RawMessage(`{"version":1}`), err
	}
	first, replayed, err := store.Mutate("ds-restore", 0, "idem-create", "dataset.created", mutation)
	if err != nil || replayed {
		t.Fatalf("首次提交失败: %s %v", first, err)
	}
	second, replayed, err := store.Mutate("ds-restore", 0, "idem-create", "dataset.created", mutation)
	if err != nil || !replayed || string(first) != string(second) {
		t.Fatalf("幂等重放失败: %s %v", second, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "projection.json"), []byte("broken"), 0o640); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := restored.Get("ds-restore")
	if err != nil || aggregate.Dataset.Version != 1 {
		t.Fatalf("事件恢复失败: %#v %v", aggregate, err)
	}
	third, replayed, err := restored.Mutate("ds-restore", 0, "idem-create", "dataset.created", mutation)
	if err != nil || !replayed || string(third) != string(first) {
		t.Fatalf("恢复后幂等结果丢失: %s %v", third, err)
	}
}

func TestStoreRejectsWrongExpectedVersion(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Mutate("missing", 2, "wrong-version", "test", func(current *domain.Aggregate, sequence int64) (*domain.Aggregate, json.RawMessage, error) {
		return nil, nil, nil
	})
	if !domain.IsCode(err, "version_conflict") {
		t.Fatalf("未返回版本冲突: %v", err)
	}
}
