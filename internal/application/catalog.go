package application

import (
	"encoding/json"
	"strconv"
	"strings"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func (s *Service) CreateDataset(input CreateDatasetInput) (MutationResult, error) {
	datasetID := stableCommandID("ds", input.IdempotencyKey)
	return s.mutate(datasetID, input.CommandMeta, "dataset.created", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current != nil {
			return nil, nil, domain.Conflict("数据集标识已存在")
		}
		aggregate, err := domain.CreateDataset(domain.NewDataset{
			ID: datasetID, Title: input.Title, SiteCode: input.SiteCode,
			CapturedFrom: input.CapturedFrom, CapturedTo: input.CapturedTo,
			TaxonomyVersion: input.TaxonomyVersion, TaxonomyCodes: input.TaxonomyCodes,
			DeviceCodes: input.DeviceCodes, Now: s.now(),
		})
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(aggregate, aggregate.Dataset)
		return aggregate, payload, err
	})
}

func (s *Service) UpdateDataset(datasetID string, input UpdateDatasetInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "dataset.metadata_updated", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		if err := current.UpdateMetadata(domain.DatasetMetadata{
			Title: input.Title, SiteCode: input.SiteCode, CapturedFrom: input.CapturedFrom,
			CapturedTo: input.CapturedTo, TaxonomyVersion: input.TaxonomyVersion,
			TaxonomyCodes: input.TaxonomyCodes, DeviceCodes: input.DeviceCodes, Now: s.now(),
		}); err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, current.Dataset)
		return current, payload, err
	})
}

func (s *Service) AddClip(datasetID string, input AddClipInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "clip.registered", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		clip, err := current.AddClip(domain.NewClip{
			ID: s.id("clip"), SourceName: input.SourceName, StartedAt: input.StartedAt,
			DurationMS: input.DurationMS, ChannelCount: input.ChannelCount, SHA256: input.SHA256,
			DeviceCode: input.DeviceCode, Metadata: input.Metadata, Now: s.now(),
		})
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, clip)
		return current, payload, err
	})
}

func (s *Service) AddClips(datasetID string, input AddClipsInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "clips.batch_registered", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		now := s.now()
		clips := make([]domain.NewClip, len(input.Clips))
		for index, item := range input.Clips {
			clips[index] = domain.NewClip{
				ID:         stableCommandID("clip", input.IdempotencyKey+":"+strconv.Itoa(index)),
				SourceName: item.SourceName, StartedAt: item.StartedAt, DurationMS: item.DurationMS,
				ChannelCount: item.ChannelCount, SHA256: item.SHA256, DeviceCode: item.DeviceCode,
				Metadata: item.Metadata, Now: now,
			}
		}
		registered, err := current.AddClips(clips, now)
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, registered)
		return current, payload, err
	})
}

func (s *Service) AddAnnotation(datasetID string, input AddAnnotationInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "annotation.revised", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		revision, err := current.AddAnnotation(domain.NewAnnotation{
			ID: s.id("ann"), ClipID: input.ClipID, StartMS: input.StartMS, EndMS: input.EndMS,
			LabelCode: input.LabelCode, Confidence: input.Confidence, Note: input.Note,
			CreatedBy: input.CreatedBy, Now: s.now(),
		})
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, revision)
		return current, payload, err
	})
}

func (s *Service) ReviseAnnotation(datasetID string, input ReviseAnnotationInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "annotation.revised", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		if strings.TrimSpace(input.ClipID) == "" {
			return nil, nil, domain.Invalid("clipId", "片段标识不能为空")
		}
		var revision domain.AnnotationRevision
		var err error
		if input.SourceRevisionID != "" {
			revision, err = current.ReviseAnnotation(stableCommandID("ann", input.IdempotencyKey), input.ClipID, input.SourceRevisionID, domain.AnnotationOverrides{
				StartMS: input.StartMS, EndMS: input.EndMS, LabelCode: input.LabelCode,
				Confidence: input.Confidence, Note: input.Note, CreatedBy: input.CreatedBy,
			}, s.now())
		} else {
			if input.StartMS == nil || input.EndMS == nil || input.LabelCode == nil || input.Confidence == nil || input.CreatedBy == nil {
				return nil, nil, domain.Invalid("annotation", "新增标注必须完整提供起止位置、分类编码、置信度和提交人")
			}
			note := ""
			if input.Note != nil {
				note = *input.Note
			}
			revision, err = current.AddAnnotation(domain.NewAnnotation{
				ID: stableCommandID("ann", input.IdempotencyKey), ClipID: input.ClipID,
				StartMS: *input.StartMS, EndMS: *input.EndMS, LabelCode: *input.LabelCode,
				Confidence: *input.Confidence, Note: note, CreatedBy: *input.CreatedBy, Now: s.now(),
			})
		}
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, revision)
		return current, payload, err
	})
}
