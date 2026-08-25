package application

import (
	"encoding/json"

	"benzhi-project-60f87529-59e6-4259-a067-c818ee931fa0/internal/domain"
)

func (s *Service) SubmitReview(datasetID string, input SubmitReviewInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "dataset.submitted", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		issues, err := current.SubmitForReview(s.now(), func() string { return s.id("issue") })
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, issues)
		return current, payload, err
	})
}

func (s *Service) AddIssue(datasetID string, input AddIssueInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "review.issue_added", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		issue, err := current.AddReviewIssue(domain.NewIssue{ID: s.id("issue"), ClipID: input.ClipID, RuleCode: input.RuleCode, Severity: input.Severity, Description: input.Description, Now: s.now()})
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, issue)
		return current, payload, err
	})
}

func (s *Service) ResolveIssue(datasetID, issueID string, input ResolveIssueInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "review.issue_resolved", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		issue, err := current.ResolveIssue(issueID, input.ResolutionRevisionID, input.ReviewedBy, s.now())
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, issue)
		return current, payload, err
	})
}

func (s *Service) ReopenIssue(datasetID, issueID string, input ReopenIssueInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "review.issue_reopened", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		issue, err := current.ReopenIssue(issueID, input.Reason, input.ReopenedBy, s.now())
		if err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, issue)
		return current, payload, err
	})
}

func (s *Service) Approve(datasetID string, input ApproveInput) (MutationResult, error) {
	return s.mutate(datasetID, input.CommandMeta, "dataset.approved", func(current *domain.Aggregate, _ int64) (*domain.Aggregate, json.RawMessage, error) {
		if current == nil {
			return nil, nil, domain.NotFound("数据集", datasetID)
		}
		if err := current.Approve(input.ReviewedBy, s.now()); err != nil {
			return nil, nil, err
		}
		payload, err := encodeResult(current, current.Dataset)
		return current, payload, err
	})
}
