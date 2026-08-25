package domain

import (
	"strings"
	"time"
)

type NewAnnotation struct {
	ID         string
	ClipID     string
	StartMS    int64
	EndMS      int64
	LabelCode  string
	Confidence float64
	Note       string
	CreatedBy  string
	Now        time.Time
}

type AnnotationOverrides struct {
	StartMS    *int64
	EndMS      *int64
	LabelCode  *string
	Confidence *float64
	Note       *string
	CreatedBy  *string
}

func (a *Aggregate) AddAnnotation(input NewAnnotation) (AnnotationRevision, error) {
	if err := a.editableForAnnotation(); err != nil {
		return AnnotationRevision{}, err
	}
	clip, exists := a.Clips[input.ClipID]
	if !exists {
		return AnnotationRevision{}, NotFound("片段", input.ClipID)
	}
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.CreatedBy) == "" {
		return AnnotationRevision{}, Invalid("createdBy", "修订标识和提交人不能为空")
	}
	if input.StartMS < 0 || input.EndMS <= input.StartMS || input.EndMS > clip.DurationMS {
		return AnnotationRevision{}, Invalid("endMs", "标注区间必须位于片段范围内且结束位置晚于开始位置")
	}
	if !contains(a.Dataset.TaxonomyCodes, input.LabelCode) {
		return AnnotationRevision{}, Invalid("labelCode", "标注分类编码不属于目标分类体系")
	}
	if input.Confidence < 0 || input.Confidence > 1 {
		return AnnotationRevision{}, Invalid("confidence", "置信度必须在 0 到 1 之间")
	}
	for _, revisions := range a.Annotations {
		for _, revision := range revisions {
			if revision.ID == input.ID {
				return AnnotationRevision{}, Conflict("标注修订标识已存在")
			}
		}
	}
	revision := AnnotationRevision{
		ID: input.ID, ClipID: input.ClipID, RevisionNo: len(a.Annotations[input.ClipID]) + 1,
		StartMS: input.StartMS, EndMS: input.EndMS, LabelCode: input.LabelCode,
		Confidence: input.Confidence, Note: strings.TrimSpace(input.Note), CreatedBy: strings.TrimSpace(input.CreatedBy),
	}
	revision.CreatedAt = input.Now.UTC()
	a.Annotations[input.ClipID] = append(a.Annotations[input.ClipID], revision)
	a.touch(input.Now)
	return revision, nil
}

func (a *Aggregate) ReviseAnnotation(id, clipID, sourceRevisionID string, overrides AnnotationOverrides, now time.Time) (AnnotationRevision, error) {
	if err := a.editableForAnnotation(); err != nil {
		return AnnotationRevision{}, err
	}
	if strings.TrimSpace(sourceRevisionID) == "" {
		return AnnotationRevision{}, Invalid("sourceRevisionId", "来源修订标识不能为空")
	}
	var source *AnnotationRevision
	for currentClipID, revisions := range a.Annotations {
		for index := range revisions {
			if revisions[index].ID == sourceRevisionID {
				if currentClipID != clipID {
					return AnnotationRevision{}, Invalid("sourceRevisionId", "来源修订不属于目标片段")
				}
				copyValue := revisions[index]
				source = &copyValue
				break
			}
		}
	}
	if source == nil {
		return AnnotationRevision{}, Invalid("sourceRevisionId", "来源修订不存在")
	}
	input := NewAnnotation{
		ID: id, ClipID: clipID, StartMS: source.StartMS, EndMS: source.EndMS,
		LabelCode: source.LabelCode, Confidence: source.Confidence, Note: source.Note,
		CreatedBy: source.CreatedBy, Now: now,
	}
	if overrides.StartMS != nil {
		input.StartMS = *overrides.StartMS
	}
	if overrides.EndMS != nil {
		input.EndMS = *overrides.EndMS
	}
	if overrides.LabelCode != nil {
		input.LabelCode = *overrides.LabelCode
	}
	if overrides.Confidence != nil {
		input.Confidence = *overrides.Confidence
	}
	if overrides.Note != nil {
		input.Note = *overrides.Note
	}
	if overrides.CreatedBy != nil {
		input.CreatedBy = *overrides.CreatedBy
	}
	return a.AddAnnotation(input)
}

func (a *Aggregate) AnnotationRevision(clipID, revisionID string) (AnnotationRevision, error) {
	if _, exists := a.Clips[clipID]; !exists {
		return AnnotationRevision{}, NotFound("片段", clipID)
	}
	for _, revisions := range a.Annotations {
		for _, revision := range revisions {
			if revision.ID == revisionID {
				if revision.ClipID != clipID {
					return AnnotationRevision{}, Invalid("revisionId", "修订不属于目标片段")
				}
				return revision, nil
			}
		}
	}
	return AnnotationRevision{}, NotFound("标注修订", revisionID)
}
