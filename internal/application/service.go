package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

type Service struct {
	store         *repository.Store
	now           func() time.Time
	id            func(string) string
	previewMu     sync.RWMutex
	manifestCache map[string]*domain.ManifestPreview
}

func NewService(store *repository.Store) *Service {
	return &Service{
		store: store, now: time.Now, id: randomID,
		manifestCache: make(map[string]*domain.ManifestPreview),
	}
}

func randomID(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(buffer)
}

func stableCommandID(prefix, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(prefix + ":" + idempotencyKey))
	return prefix + "-" + hex.EncodeToString(digest[:12])
}

func encodeResult(aggregate *domain.Aggregate, resource any) (json.RawMessage, error) {
	return json.Marshal(MutationResult{DatasetID: aggregate.Dataset.ID, Version: aggregate.Dataset.Version, Status: aggregate.Dataset.Status, Resource: resource})
}

func decodeResult(payload json.RawMessage, replayed bool) (MutationResult, error) {
	var result MutationResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return MutationResult{}, fmt.Errorf("解析持久化命令结果: %w", err)
	}
	result.Replayed = replayed
	return result, nil
}

func (s *Service) mutate(datasetID string, meta CommandMeta, action string, mutation repository.Mutation) (MutationResult, error) {
	if meta.ExpectedVersion < 0 {
		return MutationResult{}, domain.Invalid("expectedVersion", "expectedVersion 不能为负数")
	}
	if len(meta.IdempotencyKey) == 0 || len(meta.IdempotencyKey) > 128 {
		return MutationResult{}, domain.Invalid("idempotencyKey", "idempotencyKey 长度必须在 1 到 128 字符之间")
	}
	payload, replayed, err := s.store.Mutate(datasetID, meta.ExpectedVersion, meta.IdempotencyKey, action, mutation)
	if err != nil {
		return MutationResult{}, err
	}
	// 写命令提交后数据集可能已经变化（例如补交标注修订或再次批准），冻结清单预览必须
	// 反映最新批准版本与标注。缓存只对单个版本稳定，因此任何成功变更后都必须作废缓存，
	// 否则后续预览会返回首次批准时的摘要并导致冻结确认收到 state_conflict。
	s.invalidatePreviewCache(datasetID)
	return decodeResult(payload, replayed)
}
