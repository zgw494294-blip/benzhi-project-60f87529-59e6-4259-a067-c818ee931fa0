package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

type Mutation func(current *domain.Aggregate, nextReleaseSequence int64) (*domain.Aggregate, json.RawMessage, error)

type Store struct {
	mu           sync.RWMutex
	directory    string
	eventPath    string
	snapshotPath string
	state        persistedState
}

func Open(directory string) (*Store, error) {
	if directory == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	store := &Store{
		directory:    directory,
		eventPath:    filepath.Join(directory, "events.log"),
		snapshotPath: filepath.Join(directory, "projection.json"),
		state:        emptyState(),
	}
	if err := store.restore(); err != nil {
		return nil, err
	}
	return store, nil
}

func emptyState() persistedState {
	return persistedState{Datasets: make(map[string]*domain.Aggregate), Idempotency: make(map[string]IdempotencyRecord)}
}

func (s *Store) Mutate(datasetID string, expectedVersion int64, idempotencyKey, action string, mutate Mutation) (json.RawMessage, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idempotencyKey == "" {
		return nil, false, domain.Invalid("idempotencyKey", "idempotencyKey 不能为空")
	}
	if record, exists := s.state.Idempotency[idempotencyKey]; exists {
		if record.DatasetID != datasetID || record.Action != action {
			return nil, false, idempotencyConflict()
		}
		return append(json.RawMessage(nil), record.Response...), true, nil
	}
	current := s.state.Datasets[datasetID]
	actualVersion := int64(0)
	if current != nil {
		actualVersion = current.Dataset.Version
	}
	if expectedVersion != actualVersion {
		return nil, false, versionConflict(expectedVersion, actualVersion)
	}
	nextRelease := s.state.ReleaseSequence + 1
	nextAggregate, response, err := mutate(current.Clone(), nextRelease)
	if err != nil {
		return nil, false, err
	}
	if nextAggregate == nil || nextAggregate.Dataset.ID != datasetID {
		return nil, false, errors.New("提交函数返回了无效聚合")
	}
	record := IdempotencyRecord{DatasetID: datasetID, Action: action, Response: append(json.RawMessage(nil), response...), CreatedAt: time.Now().UTC()}
	frame := EventFrame{
		SchemaVersion: schemaVersion, Sequence: s.state.Sequence + 1, PreviousDigest: s.state.LastDigest,
		EventType: action, DatasetID: datasetID, OccurredAt: time.Now().UTC(), Aggregate: nextAggregate.Clone(),
		IdempotencyKey: idempotencyKey, Idempotency: record,
	}
	if action == "dataset.released" {
		if nextAggregate.Credential == nil || nextAggregate.Credential.Sequence != nextRelease {
			return nil, false, errors.New("发布凭据序号不连续")
		}
	}
	if err := s.appendFrame(&frame); err != nil {
		return nil, false, err
	}
	s.applyFrame(frame)
	if err := s.writeSnapshot(); err != nil {
		return nil, false, fmt.Errorf("事件已提交但投影写入失败: %w", err)
	}
	return response, false, nil
}

func (s *Store) Get(datasetID string) (*domain.Aggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	aggregate := s.state.Datasets[datasetID]
	if aggregate == nil {
		return nil, domain.NotFound("数据集", datasetID)
	}
	return aggregate.Clone(), nil
}

func (s *Store) List() []*domain.Aggregate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.state.Datasets))
	for id := range s.state.Datasets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*domain.Aggregate, 0, len(ids))
	for _, id := range ids {
		result = append(result, s.state.Datasets[id].Clone())
	}
	return result
}

func (s *Store) Credential(credentialID string) (*domain.ReleaseCredential, *domain.FrozenManifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, aggregate := range s.state.Datasets {
		if aggregate.Credential != nil && aggregate.Credential.ID == credentialID {
			credential := *aggregate.Credential
			manifest := *aggregate.Manifest
			manifest.Clips = append([]domain.ManifestClip(nil), aggregate.Manifest.Clips...)
			return &credential, &manifest, nil
		}
	}
	return nil, nil, domain.NotFound("发布凭据", credentialID)
}

func (s *Store) applyFrame(frame EventFrame) {
	s.state.Sequence = frame.Sequence
	s.state.LastDigest = frame.Digest
	s.state.Datasets[frame.DatasetID] = frame.Aggregate.Clone()
	s.state.Idempotency[frame.IdempotencyKey] = frame.Idempotency
	if frame.Aggregate.Credential != nil && frame.Aggregate.Credential.Sequence > s.state.ReleaseSequence {
		s.state.ReleaseSequence = frame.Aggregate.Credential.Sequence
	}
}
