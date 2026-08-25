package repository

import (
	"encoding/json"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

const schemaVersion = 1

type IdempotencyRecord struct {
	DatasetID string          `json:"datasetId"`
	Action    string          `json:"action"`
	Response  json.RawMessage `json:"response"`
	CreatedAt time.Time       `json:"createdAt"`
}

type EventFrame struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Sequence       int64             `json:"sequence"`
	PreviousDigest string            `json:"previousDigest"`
	Digest         string            `json:"digest"`
	EventType      string            `json:"eventType"`
	DatasetID      string            `json:"datasetId"`
	OccurredAt     time.Time         `json:"occurredAt"`
	Aggregate      *domain.Aggregate `json:"aggregate"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Idempotency    IdempotencyRecord `json:"idempotency"`
}

type persistedState struct {
	Sequence        int64                        `json:"sequence"`
	LastDigest      string                       `json:"lastDigest"`
	ReleaseSequence int64                        `json:"releaseSequence"`
	Datasets        map[string]*domain.Aggregate `json:"datasets"`
	Idempotency     map[string]IdempotencyRecord `json:"idempotency"`
}

type snapshotEnvelope struct {
	SchemaVersion int            `json:"schemaVersion"`
	State         persistedState `json:"state"`
	Digest        string         `json:"digest"`
}
