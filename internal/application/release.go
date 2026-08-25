package application

import (
	"encoding/json"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func (s *Service) Freeze(datasetID string, input FreezeInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "dataset.frozen", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		manifest, err := current.FreezeConfirmed(input.PreviewDigest, s.now())
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, manifest)
		return current, payload, err
	})
}

func (s *Service) PreviewManifest(datasetID string) (*domain.ManifestPreview, error) {
	current, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	return current.PreviewManifest()
}

func (s *Service) Release(datasetID string, input ReleaseInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "dataset.released", func(current *domain.Aggregate, sequence int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		credential, err := current.Release(s.id("cred"), sequence, input.IssuedBy, s.now())
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, credential)
		return current, payload, err
	})
}
