package config

import (
	"strings"

	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

// planPreview is the command-layer Terraform-style split view used by
// `config plan` and `config apply --dry-run`.
type planPreview struct {
	Organization      string              `json:"organization"`
	PlanSummary       gitopsplan.Summary  `json:"plan_summary"`
	ExecutableActions []gitopsplan.Action `json:"executable_actions"`
	SkippedActions    []gitopsplan.Action `json:"skipped_actions"`
}

func (p *planPreview) Normalize() {
	if p == nil {
		return
	}
	if p.ExecutableActions == nil {
		p.ExecutableActions = []gitopsplan.Action{}
	}
	if p.SkippedActions == nil {
		p.SkippedActions = []gitopsplan.Action{}
	}
	for i := range p.ExecutableActions {
		p.ExecutableActions[i].Normalize()
	}
	for i := range p.SkippedActions {
		p.SkippedActions[i].Normalize()
	}
}

func previewFromPlan(report *gitopsplan.Report) *planPreview {
	if report == nil {
		return &planPreview{}
	}
	preview := &planPreview{
		Organization: strings.TrimSpace(report.Organization),
		PlanSummary:  report.Summary,
	}
	for _, action := range report.Actions {
		if action.Executable {
			preview.ExecutableActions = append(preview.ExecutableActions, action)
			continue
		}
		preview.SkippedActions = append(preview.SkippedActions, action)
	}
	return preview
}
