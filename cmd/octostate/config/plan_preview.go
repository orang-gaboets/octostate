package config

import (
	"strings"

	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// planPreview is the command-layer Terraform-style split view used by
// `config plan` and `config apply --dry-run`.
type planPreview struct {
	Organization              string              `json:"organization"`
	DesiredConfigIdentity     string              `json:"desired_config_identity"`
	PlanSummary               gitopsplan.Summary  `json:"plan_summary"`
	ExecutableActions         []gitopsplan.Action `json:"executable_actions"`
	SkippedActions            []gitopsplan.Action `json:"skipped_actions"`
	UnresolvedTeamMemberships []state.TeamMember  `json:"unresolved_team_memberships"`
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
	if p.UnresolvedTeamMemberships == nil {
		p.UnresolvedTeamMemberships = []state.TeamMember{}
	}
	unresolved := state.OrganizationState{TeamMembers: p.UnresolvedTeamMemberships}
	unresolved.Normalize()
	p.UnresolvedTeamMemberships = unresolved.TeamMembers
}

func previewFromPlan(report *gitopsplan.Report) *planPreview {
	if report == nil {
		return &planPreview{
			ExecutableActions:         []gitopsplan.Action{},
			SkippedActions:            []gitopsplan.Action{},
			UnresolvedTeamMemberships: []state.TeamMember{},
		}
	}
	preview := &planPreview{
		Organization:              strings.TrimSpace(report.Organization),
		PlanSummary:               report.Summary,
		ExecutableActions:         []gitopsplan.Action{},
		SkippedActions:            []gitopsplan.Action{},
		UnresolvedTeamMemberships: append([]state.TeamMember(nil), report.UnresolvedTeamMemberships...),
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
