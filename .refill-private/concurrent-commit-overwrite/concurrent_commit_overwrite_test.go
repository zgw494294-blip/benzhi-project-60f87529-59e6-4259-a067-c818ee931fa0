package concurrent_commit_overwrite_test

import (
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestConcurrentMutationsCannotBothCommitFromSameVersion(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	_, _, err = store.Mutate("ds-concurrent", 0, "create-concurrent", "dataset.created", func(_ *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		aggregate, createErr := domain.CreateDataset(domain.NewDataset{
			ID: "ds-concurrent", Title: "并发基线", SiteCode: "SITE-1",
			CapturedFrom: now, CapturedTo: now.Add(time.Hour), TaxonomyVersion: "v1",
			TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"REC-1"}, Now: now,
		})
		return aggregate, json.RawMessage(`{"version":1}`), createErr
	})
	if err != nil {
		t.Fatalf("创建基线数据集失败: %v", err)
	}

	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	type outcome struct{ err error }
	results := make(chan outcome, 2)
	mutateTitle := func(title, key string) {
		_, _, mutationErr := store.Mutate("ds-concurrent", 1, key, "dataset.metadata_updated", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
			ready <- struct{}{}
			<-release
			updateErr := current.UpdateMetadata(domain.DatasetMetadata{
				Title: title, SiteCode: "SITE-1", CapturedFrom: now, CapturedTo: now.Add(time.Hour),
				TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"REC-1"}, Now: now.Add(time.Minute),
			})
			return current, json.RawMessage(`{"version":2}`), updateErr
		})
		results <- outcome{err: mutationErr}
	}

	go mutateTitle("并发标题 A", "update-a")
	go mutateTitle("并发标题 B", "update-b")
	<-ready
	<-ready
	close(release)

	conflicts := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if domain.IsCode(result.err, "version_conflict") {
			conflicts++
			continue
		}
		if result.err != nil {
			t.Fatalf("并发提交返回了非预期错误: %v", result.err)
		}
	}
	if conflicts != 1 {
		t.Fatalf("TestConcurrentMutationsCannotBothCommitFromSameVersion: 同一 expectedVersion 的两个命令均提交成功，实际版本冲突数为 %d", conflicts)
	}

	aggregate, err := store.Get("ds-concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.Dataset.Version != 2 {
		t.Fatalf("并发乐观锁应只允许版本递增一次，得到 %d", aggregate.Dataset.Version)
	}
	history, err := store.History("ds-concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("事件日志应仅包含创建和一次更新，得到 %d 帧", len(history))
	}
}
