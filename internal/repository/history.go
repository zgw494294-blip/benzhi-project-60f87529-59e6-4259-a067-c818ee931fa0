package repository

import (
	"sort"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

type EventRecord struct {
	Sequence       int64         `json:"sequence"`
	EventType      string        `json:"eventType"`
	DatasetID      string        `json:"datasetId"`
	DatasetVersion int64         `json:"datasetVersion"`
	Status         domain.Status `json:"status"`
	OccurredAt     time.Time     `json:"occurredAt"`
	Digest         string        `json:"digest"`
	PreviousDigest string        `json:"previousDigest"`
}

func (s *Store) History(datasetID string) ([]EventRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.Datasets[datasetID] == nil {
		return nil, domain.NotFound("数据集", datasetID)
	}
	frames, _, err := readFrames(s.eventPath)
	if err != nil {
		return nil, err
	}
	result := make([]EventRecord, 0)
	for _, frame := range frames {
		if frame.DatasetID != datasetID {
			continue
		}
		result = append(result, EventRecord{
			Sequence: frame.Sequence, EventType: frame.EventType, DatasetID: frame.DatasetID,
			DatasetVersion: frame.Aggregate.Dataset.Version, Status: frame.Aggregate.Dataset.Status,
			OccurredAt: frame.OccurredAt, Digest: frame.Digest, PreviousDigest: frame.PreviousDigest,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Sequence < result[j].Sequence })
	return result, nil
}
