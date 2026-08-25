package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/repository"
)

func (s *Service) Dataset(datasetID string) (*domain.Aggregate, error) {
	return s.store.Get(datasetID)
}

func (s *Service) ClipCatalog(datasetID string, query ClipCatalogQuery) (*ClipCatalogView, error) {
	aggregate, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	if query.Page < 1 {
		return nil, domain.Invalid("page", "页码必须为正整数")
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		return nil, domain.Invalid("pageSize", "每页条数必须在 1 到 100 之间")
	}
	clips, err := aggregate.FilterClips(domain.ClipFilter{
		SourceName: query.SourceName, DeviceCode: query.DeviceCode, StartedFrom: query.StartedFrom,
		StartedTo: query.StartedTo, Habitat: query.Habitat,
	})
	if err != nil {
		return nil, err
	}
	stats := ClipCatalogStats{Scope: "current_filter", MatchedCount: len(clips), ByDevice: make(map[string]int), ByHabitat: make(map[string]int)}
	for _, clip := range clips {
		stats.TotalDurationMS += clip.DurationMS
		stats.ByDevice[clip.DeviceCode]++
		stats.ByHabitat[clip.Metadata["habitat"]]++
	}
	start := len(clips)
	if query.Page-1 <= len(clips)/query.PageSize {
		start = (query.Page - 1) * query.PageSize
	}
	end := start + query.PageSize
	if end > len(clips) {
		end = len(clips)
	}
	items := make([]ClipCatalogItem, 0, end-start)
	for _, clip := range clips[start:end] {
		count := len(aggregate.Annotations[clip.ID])
		items = append(items, ClipCatalogItem{AudioClip: clip, RevisionCount: count, Unannotated: count == 0})
	}
	return &ClipCatalogView{DatasetID: datasetID, Version: aggregate.Dataset.Version, Page: query.Page, PageSize: query.PageSize, Items: items, Stats: stats}, nil
}

func (s *Service) ReviewPreflight(datasetID string) (*ReviewPreflightView, error) {
	aggregate, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	findings, err := aggregate.ReviewPreflight()
	if err != nil {
		return nil, err
	}
	view := &ReviewPreflightView{DatasetID: datasetID, Version: aggregate.Dataset.Version, Groups: make([]ReviewFindingGroup, 0)}
	for _, finding := range findings {
		if finding.Severity == "blocking" {
			view.Blocking++
		} else {
			view.Advisory++
		}
		last := len(view.Groups) - 1
		if last < 0 || view.Groups[last].RuleCode != finding.RuleCode || view.Groups[last].ClipID != finding.ClipID {
			view.Groups = append(view.Groups, ReviewFindingGroup{RuleCode: finding.RuleCode, ClipID: finding.ClipID, SourceName: finding.SourceName, Description: finding.Description})
			last++
		}
		view.Groups[last].Findings = append(view.Groups[last].Findings, finding)
	}
	sort.SliceStable(view.Groups, func(i, j int) bool {
		if view.Groups[i].RuleCode != view.Groups[j].RuleCode {
			return view.Groups[i].RuleCode < view.Groups[j].RuleCode
		}
		return view.Groups[i].ClipID < view.Groups[j].ClipID
	})
	return view, nil
}

func (s *Service) CompareAnnotations(datasetID, clipID, leftID, rightID string) (*AnnotationComparison, error) {
	aggregate, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	if clipID == "" || leftID == "" || rightID == "" {
		return nil, domain.Invalid("revisionId", "片段和两个修订标识均不能为空")
	}
	left, err := aggregate.AnnotationRevision(clipID, leftID)
	if err != nil {
		return nil, err
	}
	right, err := aggregate.AnnotationRevision(clipID, rightID)
	if err != nil {
		return nil, err
	}
	changed := make([]string, 0, 6)
	if left.StartMS != right.StartMS {
		changed = append(changed, "startMs")
	}
	if left.EndMS != right.EndMS {
		changed = append(changed, "endMs")
	}
	if left.LabelCode != right.LabelCode {
		changed = append(changed, "labelCode")
	}
	if left.Confidence != right.Confidence {
		changed = append(changed, "confidence")
	}
	if left.Note != right.Note {
		changed = append(changed, "note")
	}
	if left.CreatedBy != right.CreatedBy {
		changed = append(changed, "createdBy")
	}
	slices.Sort(changed)
	return &AnnotationComparison{ClipID: clipID, Left: left, Right: right, ChangedFields: changed}, nil
}

func (s *Service) History(datasetID string) ([]repository.EventRecord, error) {
	return s.store.History(datasetID)
}

func (s *Service) Datasets() []DatasetSummary {
	aggregates := s.store.List()
	result := make([]DatasetSummary, 0, len(aggregates))
	for _, aggregate := range aggregates {
		open := 0
		for _, issue := range aggregate.Issues {
			if issue.Status == domain.IssueOpen {
				open++
			}
		}
		result = append(result, DatasetSummary{
			ID: aggregate.Dataset.ID, Title: aggregate.Dataset.Title, SiteCode: aggregate.Dataset.SiteCode,
			Status: aggregate.Dataset.Status, Version: aggregate.Dataset.Version,
			ClipCount: len(aggregate.Clips), OpenIssues: open, UpdatedAt: aggregate.Dataset.UpdatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result
}

func (s *Service) Manifest(datasetID string) (*domain.FrozenManifest, error) {
	aggregate, err := s.store.Get(datasetID)
	if err != nil {
		return nil, err
	}
	if aggregate.Manifest == nil {
		return nil, domain.NotFound("冻结清单", datasetID)
	}
	return aggregate.Manifest, nil
}

func (s *Service) VerifyCredential(credentialID string) (*VerificationView, error) {
	s.verificationMu.RLock()
	cached := s.verificationCache[credentialID]
	s.verificationMu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	credential, manifest, err := s.store.Credential(credentialID)
	if err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(struct {
		DatasetID      string                `json:"datasetId"`
		DatasetVersion int64                 `json:"datasetVersion"`
		Clips          []domain.ManifestClip `json:"clips"`
	}{manifest.DatasetID, manifest.DatasetVersion, manifest.Clips})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	actual := hex.EncodeToString(digest[:])
	verified := actual == manifest.Digest && credential.ManifestDigest == manifest.Digest && credential.DatasetVersion == manifest.DatasetVersion
	view := &VerificationView{Credential: *credential, Manifest: *manifest, Verified: verified}
	s.verificationMu.Lock()
	s.verificationCache[credentialID] = view
	s.verificationMu.Unlock()
	return view, nil
}
