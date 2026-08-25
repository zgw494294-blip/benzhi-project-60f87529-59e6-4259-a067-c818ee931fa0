package application

import (
	"time"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type CreateDatasetInput struct {
	CommandMeta
	Title           string    `json:"title"`
	SiteCode        string    `json:"siteCode"`
	CapturedFrom    time.Time `json:"capturedFrom"`
	CapturedTo      time.Time `json:"capturedTo"`
	TaxonomyVersion string    `json:"taxonomyVersion"`
	TaxonomyCodes   []string  `json:"taxonomyCodes"`
	DeviceCodes     []string  `json:"deviceCodes"`
}

type AddClipInput struct {
	CommandMeta
	SourceName   string            `json:"sourceName"`
	StartedAt    time.Time         `json:"startedAt"`
	DurationMS   int64             `json:"durationMs"`
	ChannelCount int               `json:"channelCount"`
	SHA256       string            `json:"sha256"`
	DeviceCode   string            `json:"deviceCode"`
	Metadata     map[string]string `json:"metadata"`
}

type ClipRecordInput struct {
	SourceName   string            `json:"sourceName"`
	StartedAt    time.Time         `json:"startedAt"`
	DurationMS   int64             `json:"durationMs"`
	ChannelCount int               `json:"channelCount"`
	SHA256       string            `json:"sha256"`
	DeviceCode   string            `json:"deviceCode"`
	Metadata     map[string]string `json:"metadata"`
}

type AddClipsInput struct {
	CommandMeta
	Clips []ClipRecordInput `json:"clips"`
}

type UpdateDatasetInput struct {
	CommandMeta
	Title           string    `json:"title"`
	SiteCode        string    `json:"siteCode"`
	CapturedFrom    time.Time `json:"capturedFrom"`
	CapturedTo      time.Time `json:"capturedTo"`
	TaxonomyVersion string    `json:"taxonomyVersion"`
	TaxonomyCodes   []string  `json:"taxonomyCodes"`
	DeviceCodes     []string  `json:"deviceCodes"`
}

type AddAnnotationInput struct {
	CommandMeta
	ClipID     string  `json:"clipId"`
	StartMS    int64   `json:"startMs"`
	EndMS      int64   `json:"endMs"`
	LabelCode  string  `json:"labelCode"`
	Confidence float64 `json:"confidence"`
	Note       string  `json:"note"`
	CreatedBy  string  `json:"createdBy"`
}

type ReviseAnnotationInput struct {
	CommandMeta
	ClipID           string   `json:"clipId"`
	SourceRevisionID string   `json:"sourceRevisionId,omitempty"`
	StartMS          *int64   `json:"startMs,omitempty"`
	EndMS            *int64   `json:"endMs,omitempty"`
	LabelCode        *string  `json:"labelCode,omitempty"`
	Confidence       *float64 `json:"confidence,omitempty"`
	Note             *string  `json:"note,omitempty"`
	CreatedBy        *string  `json:"createdBy,omitempty"`
}

type SubmitReviewInput struct{ CommandMeta }

type AddIssueInput struct {
	CommandMeta
	ClipID      string `json:"clipId"`
	RuleCode    string `json:"ruleCode"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type ResolveIssueInput struct {
	CommandMeta
	ResolutionRevisionID string `json:"resolutionRevisionId"`
	ReviewedBy           string `json:"reviewedBy"`
}

type ReopenIssueInput struct {
	CommandMeta
	Reason     string `json:"reason"`
	ReopenedBy string `json:"reopenedBy"`
}

type ApproveInput struct {
	CommandMeta
	ReviewedBy string `json:"reviewedBy"`
}

type FreezeInput struct {
	CommandMeta
	PreviewDigest string `json:"previewDigest"`
}

type ReleaseInput struct {
	CommandMeta
	IssuedBy string `json:"issuedBy"`
}

type MutationResult struct {
	DatasetID string        `json:"datasetId"`
	Version   int64         `json:"version"`
	Status    domain.Status `json:"status"`
	Resource  any           `json:"resource,omitempty"`
	Replayed  bool          `json:"replayed,omitempty"`
}

type DatasetSummary struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	SiteCode   string        `json:"siteCode"`
	Status     domain.Status `json:"status"`
	Version    int64         `json:"version"`
	ClipCount  int           `json:"clipCount"`
	OpenIssues int           `json:"openIssues"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

type VerificationView struct {
	Credential domain.ReleaseCredential `json:"credential"`
	Manifest   domain.FrozenManifest    `json:"manifest"`
	Verified   bool                     `json:"verified"`
}

type ClipCatalogQuery struct {
	SourceName  string
	DeviceCode  string
	StartedFrom *time.Time
	StartedTo   *time.Time
	Habitat     string
	Page        int
	PageSize    int
}

type ClipCatalogItem struct {
	domain.AudioClip
	RevisionCount int  `json:"revisionCount"`
	Unannotated   bool `json:"unannotated"`
}

type ClipCatalogStats struct {
	Scope           string         `json:"scope"`
	MatchedCount    int            `json:"matchedCount"`
	TotalDurationMS int64          `json:"totalDurationMs"`
	ByDevice        map[string]int `json:"byDevice"`
	ByHabitat       map[string]int `json:"byHabitat"`
}

type ClipCatalogView struct {
	DatasetID string            `json:"datasetId"`
	Version   int64             `json:"version"`
	Page      int               `json:"page"`
	PageSize  int               `json:"pageSize"`
	Items     []ClipCatalogItem `json:"items"`
	Stats     ClipCatalogStats  `json:"stats"`
}

type ReviewFindingGroup struct {
	RuleCode    string                 `json:"ruleCode"`
	ClipID      string                 `json:"clipId,omitempty"`
	SourceName  string                 `json:"sourceName,omitempty"`
	Description string                 `json:"description"`
	Findings    []domain.ReviewFinding `json:"findings"`
}

type ReviewPreflightView struct {
	DatasetID string               `json:"datasetId"`
	Version   int64                `json:"version"`
	Blocking  int                  `json:"blocking"`
	Advisory  int                  `json:"advisory"`
	Groups    []ReviewFindingGroup `json:"groups"`
}

type AnnotationComparison struct {
	ClipID        string                    `json:"clipId"`
	Left          domain.AnnotationRevision `json:"left"`
	Right         domain.AnnotationRevision `json:"right"`
	ChangedFields []string                  `json:"changedFields"`
}
