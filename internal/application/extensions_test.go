package application

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestBatchIdempotencyCatalogAndPreflightAreConsistent(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	now := time.Now().UTC().Truncate(time.Second)
	created, err := service.CreateDataset(CreateDatasetInput{
		CommandMeta: CommandMeta{ExpectedVersion: 0, IdempotencyKey: "create-batch-test"},
		Title:       "批量测试", SiteCode: "S", CapturedFrom: now, CapturedTo: now.Add(time.Hour),
		TaxonomyVersion: "v1", TaxonomyCodes: []string{"bird.a"}, DeviceCodes: []string{"R1", "R2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := AddClipsInput{CommandMeta: CommandMeta{ExpectedVersion: created.Version, IdempotencyKey: "batch-once"}, Clips: []ClipRecordInput{
		{SourceName: "one.wav", StartedAt: now.Add(time.Minute), DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "R1", Metadata: map[string]string{"habitat": "forest"}},
		{SourceName: "two.wav", StartedAt: now.Add(2 * time.Minute), DurationMS: 2000, ChannelCount: 1, SHA256: strings.Repeat("b", 64), DeviceCode: "R2", Metadata: map[string]string{"habitat": "wetland"}},
		{SourceName: "three.wav", StartedAt: now.Add(3 * time.Minute), DurationMS: 3000, ChannelCount: 2, SHA256: strings.Repeat("c", 64), DeviceCode: "R1", Metadata: map[string]string{"habitat": "forest"}},
	}}
	result, err := service.AddClips(created.DatasetID, input)
	if err != nil || result.Version != created.Version+1 {
		t.Fatalf("批量提交失败: %#v %v", result, err)
	}
	var clips []domain.AudioClip
	payload, _ := json.Marshal(result.Resource)
	if err := json.Unmarshal(payload, &clips); err != nil || len(clips) != 3 {
		t.Fatalf("批量回执无效: %#v %v", result.Resource, err)
	}
	replayed, err := service.AddClips(created.DatasetID, input)
	if err != nil || !replayed.Replayed || replayed.Version != result.Version {
		t.Fatalf("批量幂等回放失败: %#v %v", replayed, err)
	}
	history, err := service.History(created.DatasetID)
	if err != nil || len(history) != 2 {
		t.Fatalf("批量命令未保持单事件: %d %v", len(history), err)
	}
	view, err := service.ClipCatalog(created.DatasetID, ClipCatalogQuery{DeviceCode: "R1", Habitat: "forest", Page: 1, PageSize: 1})
	if err != nil || view.Stats.MatchedCount != 2 || view.Stats.TotalDurationMS != 4000 || len(view.Items) != 1 || view.Stats.ByDevice["R1"] != 2 {
		t.Fatalf("组合目录或统计错误: %#v %v", view, err)
	}
	preflight, err := service.ReviewPreflight(created.DatasetID)
	if err != nil || preflight.Blocking != 3 || preflight.Version != result.Version {
		t.Fatalf("预检结果错误: %#v %v", preflight, err)
	}
	after, err := service.History(created.DatasetID)
	if err != nil || len(after) != len(history) {
		t.Fatalf("只读查询追加了事件: %d -> %d, %v", len(history), len(after), err)
	}
	invalid := AddClipsInput{CommandMeta: CommandMeta{ExpectedVersion: result.Version, IdempotencyKey: "batch-invalid"}, Clips: []ClipRecordInput{
		{SourceName: "four.wav", StartedAt: now.Add(4 * time.Minute), DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("d", 64), DeviceCode: "R1", Metadata: map[string]string{"habitat": "forest"}},
		{SourceName: "duplicate.wav", StartedAt: now.Add(5 * time.Minute), DurationMS: 1000, ChannelCount: 1, SHA256: strings.Repeat("a", 64), DeviceCode: "R1", Metadata: map[string]string{"habitat": "forest"}},
	}}
	if _, err := service.AddClips(created.DatasetID, invalid); !domain.IsCode(err, "validation_failed") {
		t.Fatalf("非法批次未拒绝: %v", err)
	}
	aggregate, err := service.Dataset(created.DatasetID)
	if err != nil || aggregate.Dataset.Version != result.Version || len(aggregate.Clips) != 3 {
		t.Fatalf("非法批次发生部分落库: %#v %v", aggregate, err)
	}
}
