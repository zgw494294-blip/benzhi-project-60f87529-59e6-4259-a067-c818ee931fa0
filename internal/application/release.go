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
	s.previewMu.RLock()
	cached := s.manifestCache[datasetID]
	s.previewMu.RUnlock()
	if cached != nil {
		return cloneManifestPreview(cached), nil
	}
	current, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	preview, err := current.PreviewManifest()
	if err != nil {
		return nil, err
	}
	s.previewMu.Lock()
	s.manifestCache[datasetID] = cloneManifestPreview(preview)
	s.previewMu.Unlock()
	return preview, nil
}

func (s *Service) invalidatePreviewCache(datasetID string) {
	s.previewMu.Lock()
	delete(s.manifestCache, datasetID)
	s.previewMu.Unlock()
}

func cloneManifestPreview(input *domain.ManifestPreview) *domain.ManifestPreview {
	if input == nil {
		return nil
	}
	copyValue := *input
	copyValue.RevisionsByLabel = make(map[string]int, len(input.RevisionsByLabel))
	for label, count := range input.RevisionsByLabel {
		copyValue.RevisionsByLabel[label] = count
	}
	copyValue.Clips = append([]domain.ManifestClip(nil), input.Clips...)
	for index := range copyValue.Clips {
		copyValue.Clips[index].Annotations = append([]domain.AnnotationRevision(nil), input.Clips[index].Annotations...)
	}
	return &copyValue
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
