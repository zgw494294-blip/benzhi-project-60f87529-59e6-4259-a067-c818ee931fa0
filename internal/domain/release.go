package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func (a *Aggregate) PreviewManifest() (*ManifestPreview, error) {
	if a.Dataset.Status != StatusApproved {
		return nil, Conflict("只有 approved 数据集可预览冻结清单")
	}
	clips, digest, err := a.buildManifestCandidate()
	if err != nil {
		return nil, err
	}
	byLabel := make(map[string]int)
	revisionCount := 0
	for _, clip := range clips {
		for _, revision := range clip.Annotations {
			byLabel[revision.LabelCode]++
			revisionCount++
		}
	}
	return &ManifestPreview{
		DatasetID: a.Dataset.ID, BaseVersion: a.Dataset.Version, CandidateVersion: a.Dataset.Version + 1,
		Digest: digest, ClipCount: len(clips), AnnotationRevisionCount: revisionCount,
		RevisionsByLabel: byLabel, Clips: clips,
	}, nil
}

func (a *Aggregate) buildManifestCandidate() ([]ManifestClip, string, error) {
	clips := make([]ManifestClip, 0, len(a.Clips))
	for _, clip := range a.Clips {
		revisions := append([]AnnotationRevision(nil), a.Annotations[clip.ID]...)
		sort.Slice(revisions, func(i, j int) bool {
			if revisions[i].RevisionNo == revisions[j].RevisionNo {
				return revisions[i].ID < revisions[j].ID
			}
			return revisions[i].RevisionNo < revisions[j].RevisionNo
		})
		clips = append(clips, ManifestClip{ClipID: clip.ID, SourceName: clip.SourceName, SHA256: clip.SHA256, Annotations: revisions})
	}
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].SourceName == clips[j].SourceName {
			return clips[i].ClipID < clips[j].ClipID
		}
		return clips[i].SourceName < clips[j].SourceName
	})
	canonical, err := json.Marshal(struct {
		DatasetID      string         `json:"datasetId"`
		DatasetVersion int64          `json:"datasetVersion"`
		Clips          []ManifestClip `json:"clips"`
	}{a.Dataset.ID, a.Dataset.Version + 1, clips})
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(canonical)
	return clips, hex.EncodeToString(digest[:]), nil
}

func (a *Aggregate) FreezeConfirmed(previewDigest string, now time.Time) (*FrozenManifest, error) {
	preview, err := a.PreviewManifest()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(previewDigest) == "" {
		return nil, Invalid("previewDigest", "冻结前必须确认候选清单摘要")
	}
	if !strings.EqualFold(strings.TrimSpace(previewDigest), preview.Digest) {
		return nil, Conflict("候选清单摘要已变化，请重新预览后确认")
	}
	manifest := &FrozenManifest{DatasetID: a.Dataset.ID, DatasetVersion: preview.CandidateVersion, GeneratedAt: now.UTC(), Clips: preview.Clips, Digest: preview.Digest}
	a.Manifest = manifest
	a.Dataset.Status = StatusFrozen
	a.touch(now)
	copyValue := *manifest
	return &copyValue, nil
}

// Freeze 保留领域内的便捷调用；公开冻结入口始终使用 FreezeConfirmed 强制摘要确认。
func (a *Aggregate) Freeze(now time.Time) (*FrozenManifest, error) {
	preview, err := a.PreviewManifest()
	if err != nil {
		return nil, err
	}
	return a.FreezeConfirmed(preview.Digest, now)
}

func (a *Aggregate) Release(id string, sequence int64, issuedBy string, now time.Time) (*ReleaseCredential, error) {
	if a.Dataset.Status != StatusFrozen || a.Manifest == nil {
		return nil, Conflict("只有已生成清单的 frozen 数据集可发布")
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(issuedBy) == "" {
		return nil, Invalid("issuedBy", "凭据标识和签发人不能为空")
	}
	if sequence <= 0 {
		return nil, Invalid("sequence", "凭据序号必须为正整数")
	}
	credential := &ReleaseCredential{
		ID: id, DatasetID: a.Dataset.ID, Sequence: sequence, ManifestDigest: a.Manifest.Digest,
		DatasetVersion: a.Manifest.DatasetVersion, IssuedBy: strings.TrimSpace(issuedBy), IssuedAt: now.UTC(),
	}
	a.Credential = credential
	a.Dataset.Status = StatusReleased
	a.touch(now)
	copyValue := *credential
	return &copyValue, nil
}
