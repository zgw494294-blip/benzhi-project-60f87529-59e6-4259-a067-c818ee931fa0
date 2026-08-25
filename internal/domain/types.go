package domain

import "time"

type Status string

const (
	StatusDraft       Status = "draft"
	StatusInReview    Status = "in_review"
	StatusRemediation Status = "remediation"
	StatusApproved    Status = "approved"
	StatusFrozen      Status = "frozen"
	StatusReleased    Status = "released"
)

type Dataset struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	SiteCode        string    `json:"siteCode"`
	CapturedFrom    time.Time `json:"capturedFrom"`
	CapturedTo      time.Time `json:"capturedTo"`
	TaxonomyVersion string    `json:"taxonomyVersion"`
	TaxonomyCodes   []string  `json:"taxonomyCodes"`
	DeviceCodes     []string  `json:"deviceCodes"`
	Status          Status    `json:"status"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AudioClip struct {
	ID           string            `json:"id"`
	DatasetID    string            `json:"datasetId"`
	SourceName   string            `json:"sourceName"`
	StartedAt    time.Time         `json:"startedAt"`
	DurationMS   int64             `json:"durationMs"`
	ChannelCount int               `json:"channelCount"`
	SHA256       string            `json:"sha256"`
	DeviceCode   string            `json:"deviceCode"`
	Metadata     map[string]string `json:"metadata"`
}

type AnnotationRevision struct {
	ID         string    `json:"id"`
	ClipID     string    `json:"clipId"`
	RevisionNo int       `json:"revisionNo"`
	StartMS    int64     `json:"startMs"`
	EndMS      int64     `json:"endMs"`
	LabelCode  string    `json:"labelCode"`
	Confidence float64   `json:"confidence"`
	Note       string    `json:"note"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

type IssueStatus string

const (
	IssueOpen     IssueStatus = "open"
	IssueResolved IssueStatus = "resolved"
)

type ReviewIssue struct {
	ID                   string          `json:"id"`
	DatasetID            string          `json:"datasetId"`
	ClipID               string          `json:"clipId,omitempty"`
	RuleCode             string          `json:"ruleCode"`
	Severity             string          `json:"severity"`
	Description          string          `json:"description"`
	Status               IssueStatus     `json:"status"`
	ResolutionRevisionID string          `json:"resolutionRevisionId,omitempty"`
	ReviewedBy           string          `json:"reviewedBy,omitempty"`
	ResolvedAt           *time.Time      `json:"resolvedAt,omitempty"`
	DecisionTrail        []IssueDecision `json:"decisionTrail,omitempty"`
}

type IssueDecision struct {
	Action               string    `json:"action"`
	ResolutionRevisionID string    `json:"resolutionRevisionId,omitempty"`
	Actor                string    `json:"actor"`
	Reason               string    `json:"reason,omitempty"`
	OccurredAt           time.Time `json:"occurredAt"`
}

type ManifestClip struct {
	ClipID      string               `json:"clipId"`
	SourceName  string               `json:"sourceName"`
	SHA256      string               `json:"sha256"`
	Annotations []AnnotationRevision `json:"annotations"`
}

type FrozenManifest struct {
	DatasetID      string         `json:"datasetId"`
	DatasetVersion int64          `json:"datasetVersion"`
	GeneratedAt    time.Time      `json:"generatedAt"`
	Clips          []ManifestClip `json:"clips"`
	Digest         string         `json:"digest"`
}

type ReleaseCredential struct {
	ID             string    `json:"id"`
	DatasetID      string    `json:"datasetId"`
	Sequence       int64     `json:"sequence"`
	ManifestDigest string    `json:"manifestDigest"`
	DatasetVersion int64     `json:"datasetVersion"`
	IssuedBy       string    `json:"issuedBy"`
	IssuedAt       time.Time `json:"issuedAt"`
}

type ManifestPreview struct {
	DatasetID               string         `json:"datasetId"`
	BaseVersion             int64          `json:"baseVersion"`
	CandidateVersion        int64          `json:"candidateVersion"`
	Digest                  string         `json:"digest"`
	ClipCount               int            `json:"clipCount"`
	AnnotationRevisionCount int            `json:"annotationRevisionCount"`
	RevisionsByLabel        map[string]int `json:"revisionsByLabel"`
	Clips                   []ManifestClip `json:"clips"`
}

type Aggregate struct {
	Dataset     Dataset                         `json:"dataset"`
	Clips       map[string]AudioClip            `json:"clips"`
	Annotations map[string][]AnnotationRevision `json:"annotations"`
	Issues      map[string]ReviewIssue          `json:"issues"`
	Manifest    *FrozenManifest                 `json:"manifest,omitempty"`
	Credential  *ReleaseCredential              `json:"credential,omitempty"`
}
