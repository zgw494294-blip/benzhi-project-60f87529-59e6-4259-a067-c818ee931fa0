package torneventtail

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestTornEventTailKeepsCommittedFramesRecoverable(t *testing.T) {
	directory := t.TempDir()
	store, err := repository.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	_, _, err = store.Mutate("ds-torn", 0, "torn-create", "dataset.created", func(_ *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		aggregate, createErr := domain.CreateDataset(domain.NewDataset{
			ID: "ds-torn", Title: "尾帧恢复", SiteCode: "SITE", CapturedFrom: now,
			CapturedTo: now.Add(time.Hour), TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"},
			DeviceCodes: []string{"REC-1"}, Now: now,
		})
		return aggregate, json.RawMessage(`{"version":1}`), createErr
	})
	if err != nil {
		t.Fatal(err)
	}

	logFile, err := os.OpenFile(filepath.Join(directory, "events.log"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	var incompletePrefix [8]byte
	binary.BigEndian.PutUint64(incompletePrefix[:], 128)
	if _, err := logFile.Write(incompletePrefix[:]); err != nil {
		_ = logFile.Close()
		t.Fatal(err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatal(err)
	}

	restored, err := repository.Open(directory)
	if err != nil {
		t.Fatalf("完整已提交帧被撕裂尾部阻断恢复: %v", err)
	}
	aggregate, err := restored.Get("ds-torn")
	if err != nil {
		t.Fatalf("尾部故障后已提交数据集丢失: %v", err)
	}
	_, _, err = restored.Mutate("ds-torn", aggregate.Dataset.Version, "torn-update", "dataset.metadata_updated", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		updateErr := current.UpdateMetadata(domain.DatasetMetadata{
			Title: "尾帧恢复后更新", SiteCode: "SITE", CapturedFrom: now, CapturedTo: now.Add(time.Hour),
			TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"REC-1"}, Now: now.Add(time.Minute),
		})
		return current, json.RawMessage(`{"version":2}`), updateErr
	})
	if err != nil {
		t.Fatalf("清理撕裂尾部后不能继续提交: %v", err)
	}
	if _, err := repository.Open(directory); err != nil {
		t.Fatalf("撕裂尾部恢复后的新事件无法再次恢复: %v", err)
	}
}
