package credential_verification_alias_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/application"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func TestCredentialVerificationCacheDoesNotExposeSharedState(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}

	aggregate := releasedAggregate(t)
	_, _, err = store.Mutate(aggregate.Dataset.ID, 0, "seed-release", "dataset.released", func(_ *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		return aggregate, json.RawMessage(`{"datasetId":"ds-cache"}`), nil
	})
	if err != nil {
		t.Fatalf("seed released aggregate: %v", err)
	}

	service := application.NewService(store)
	first, err := service.VerifyCredential("cred-cache")
	if err != nil {
		t.Fatalf("first verification: %v", err)
	}
	if !first.Verified {
		t.Fatal("fixture credential should verify")
	}

	first.Verified = false
	first.Manifest.Clips[0].Annotations[0].Note = "调用方污染"

	second, err := service.VerifyCredential("cred-cache")
	if err != nil {
		t.Fatalf("second verification: %v", err)
	}
	if !second.Verified || second.Manifest.Clips[0].Annotations[0].Note != "原始备注" {
		t.Fatalf("cached verification was corrupted by the previous caller: verified=%v note=%q", second.Verified, second.Manifest.Clips[0].Annotations[0].Note)
	}
}

func releasedAggregate(t *testing.T) *domain.Aggregate {
	t.Helper()
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	annotations := []domain.AnnotationRevision{{
		ID: "ann-1", ClipID: "clip-1", RevisionNo: 1, StartMS: 10, EndMS: 900,
		LabelCode: "aves.turdus", Confidence: 0.98, Note: "原始备注", CreatedBy: "annotator", CreatedAt: now,
	}}
	clips := []domain.ManifestClip{{
		ClipID: "clip-1", SourceName: "forest.wav",
		SHA256:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Annotations: annotations,
	}}
	canonical, err := json.Marshal(struct {
		DatasetID      string                `json:"datasetId"`
		DatasetVersion int64                 `json:"datasetVersion"`
		Clips          []domain.ManifestClip `json:"clips"`
	}{"ds-cache", 4, clips})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	digestBytes := sha256.Sum256(canonical)
	digest := hex.EncodeToString(digestBytes[:])
	manifest := &domain.FrozenManifest{
		DatasetID: "ds-cache", DatasetVersion: 4, GeneratedAt: now, Clips: clips, Digest: digest,
	}
	credential := &domain.ReleaseCredential{
		ID: "cred-cache", DatasetID: "ds-cache", Sequence: 1, ManifestDigest: digest,
		DatasetVersion: 4, IssuedBy: "owner", IssuedAt: now,
	}
	return &domain.Aggregate{
		Dataset: domain.Dataset{ID: "ds-cache", Title: "缓存边界", Status: domain.StatusReleased, Version: 5, CreatedAt: now, UpdatedAt: now},
		Clips:   map[string]domain.AudioClip{}, Annotations: map[string][]domain.AnnotationRevision{},
		Issues: map[string]domain.ReviewIssue{}, Manifest: manifest, Credential: credential,
	}
}
