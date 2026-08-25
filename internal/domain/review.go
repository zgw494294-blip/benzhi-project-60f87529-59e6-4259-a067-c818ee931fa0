package domain

import (
	"fmt"
	"strings"
	"time"
)

type NewIssue struct {
	ID          string
	ClipID      string
	RuleCode    string
	Severity    string
	Description string
	Now         time.Time
}

func (a *Aggregate) SubmitForReview(now time.Time, issueID func() string) ([]ReviewIssue, error) {
	if a.Dataset.Status != StatusDraft {
		return nil, Conflict("只有 draft 数据集可送审")
	}
	if len(a.Clips) == 0 {
		return nil, Conflict("至少登记一个片段后才能送审")
	}
	generated := make([]ReviewIssue, 0)
	for _, finding := range a.validateForReview() {
		generated = append(generated, ReviewIssue{
			ID: issueID(), DatasetID: a.Dataset.ID, ClipID: finding.ClipID,
			RuleCode: finding.RuleCode, Severity: finding.Severity,
			Description: finding.Description, Status: IssueOpen,
		})
	}
	for _, issue := range generated {
		a.Issues[issue.ID] = issue
	}
	a.Dataset.Status = StatusInReview
	for _, issue := range generated {
		if issue.Severity == "blocking" {
			a.Dataset.Status = StatusRemediation
			break
		}
	}
	a.touch(now)
	return generated, nil
}

func (a *Aggregate) AddReviewIssue(input NewIssue) (ReviewIssue, error) {
	if a.Dataset.Status != StatusInReview && a.Dataset.Status != StatusRemediation {
		return ReviewIssue{}, Conflict("只有 in_review 或 remediation 数据集可登记复核问题")
	}
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.RuleCode) == "" || strings.TrimSpace(input.Description) == "" {
		return ReviewIssue{}, Invalid("description", "问题标识、规则和描述不能为空")
	}
	if input.Severity != "blocking" && input.Severity != "advisory" {
		return ReviewIssue{}, Invalid("severity", "问题严重度必须是 blocking 或 advisory")
	}
	if input.ClipID != "" {
		if _, exists := a.Clips[input.ClipID]; !exists {
			return ReviewIssue{}, NotFound("片段", input.ClipID)
		}
	}
	if _, exists := a.Issues[input.ID]; exists {
		return ReviewIssue{}, Conflict("复核问题标识已存在")
	}
	issue := ReviewIssue{ID: input.ID, DatasetID: a.Dataset.ID, ClipID: input.ClipID, RuleCode: input.RuleCode, Severity: input.Severity, Description: strings.TrimSpace(input.Description), Status: IssueOpen}
	a.Issues[issue.ID] = issue
	a.Dataset.Status = StatusRemediation
	a.touch(input.Now)
	return issue, nil
}

func (a *Aggregate) ResolveIssue(issueID, revisionID, reviewer string, now time.Time) (ReviewIssue, error) {
	if a.Dataset.Status != StatusRemediation && a.Dataset.Status != StatusInReview {
		return ReviewIssue{}, Conflict("当前状态不可关闭复核问题")
	}
	issue, exists := a.Issues[issueID]
	if !exists {
		return ReviewIssue{}, NotFound("复核问题", issueID)
	}
	if issue.Status == IssueResolved {
		return ReviewIssue{}, Conflict("复核问题已经关闭")
	}
	if strings.TrimSpace(reviewer) == "" {
		return ReviewIssue{}, Invalid("reviewedBy", "复核人不能为空")
	}
	if revisionID != "" && !a.hasRevision(revisionID, issue.ClipID) {
		return ReviewIssue{}, Invalid("resolutionRevisionId", "整改修订不存在或不属于问题片段")
	}
	if issue.Severity == "blocking" && revisionID == "" && issue.ClipID != "" {
		return ReviewIssue{}, Invalid("resolutionRevisionId", "片段阻断问题必须关联整改修订")
	}
	resolvedAt := now.UTC()
	issue.Status = IssueResolved
	issue.ResolutionRevisionID = revisionID
	issue.ReviewedBy = strings.TrimSpace(reviewer)
	issue.ResolvedAt = &resolvedAt
	issue.DecisionTrail = append(issue.DecisionTrail, IssueDecision{Action: "resolved", ResolutionRevisionID: revisionID, Actor: strings.TrimSpace(reviewer), OccurredAt: resolvedAt})
	a.Issues[issueID] = issue
	a.touch(now)
	return issue, nil
}

func (a *Aggregate) ReopenIssue(issueID, reason, actor string, now time.Time) (ReviewIssue, error) {
	if a.Dataset.Status == StatusFrozen || a.Dataset.Status == StatusReleased {
		return ReviewIssue{}, Conflict("frozen 或 released 数据集中的问题不可重开")
	}
	if a.Dataset.Status != StatusInReview && a.Dataset.Status != StatusRemediation && a.Dataset.Status != StatusApproved {
		return ReviewIssue{}, Conflict("当前状态不可重开复核问题")
	}
	issue, exists := a.Issues[issueID]
	if !exists {
		return ReviewIssue{}, NotFound("复核问题", issueID)
	}
	if issue.Status != IssueResolved {
		return ReviewIssue{}, Conflict("只有已关闭问题可重新打开")
	}
	if a.Dataset.Status == StatusApproved && issue.Severity != "blocking" {
		return ReviewIssue{}, Conflict("approved 数据集只能重开阻断问题")
	}
	if strings.TrimSpace(reason) == "" {
		return ReviewIssue{}, Invalid("reason", "重开原因不能为空")
	}
	if strings.TrimSpace(actor) == "" {
		return ReviewIssue{}, Invalid("reopenedBy", "重开操作人不能为空")
	}
	occurredAt := now.UTC()
	if len(issue.DecisionTrail) == 0 && issue.ResolvedAt != nil {
		issue.DecisionTrail = append(issue.DecisionTrail, IssueDecision{Action: "resolved", ResolutionRevisionID: issue.ResolutionRevisionID, Actor: issue.ReviewedBy, OccurredAt: issue.ResolvedAt.UTC()})
	}
	issue.DecisionTrail = append(issue.DecisionTrail, IssueDecision{Action: "reopened", Actor: strings.TrimSpace(actor), Reason: strings.TrimSpace(reason), OccurredAt: occurredAt})
	issue.Status = IssueOpen
	issue.ResolutionRevisionID = ""
	issue.ReviewedBy = ""
	issue.ResolvedAt = nil
	a.Issues[issueID] = issue
	a.Dataset.Status = StatusRemediation
	a.touch(now)
	return issue, nil
}

func (a *Aggregate) hasRevision(id, clipID string) bool {
	for currentClip, revisions := range a.Annotations {
		if clipID != "" && currentClip != clipID {
			continue
		}
		for _, revision := range revisions {
			if revision.ID == id {
				return true
			}
		}
	}
	return false
}

func (a *Aggregate) Approve(reviewer string, now time.Time) error {
	if a.Dataset.Status != StatusInReview && a.Dataset.Status != StatusRemediation {
		return Conflict("只有待复核或整改中的数据集可批准")
	}
	if strings.TrimSpace(reviewer) == "" {
		return Invalid("reviewedBy", "批准人不能为空")
	}
	for _, issue := range a.Issues {
		if issue.Severity == "blocking" && issue.Status != IssueResolved {
			return Conflict(fmt.Sprintf("阻断问题 %s 尚未关闭", issue.ID))
		}
	}
	a.Dataset.Status = StatusApproved
	a.touch(now)
	return nil
}
