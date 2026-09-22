package harness

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// CollaborationMode — plan | build | edit | yolo | auto. Port of
// core/src/permission/service.ts. Note the deliberate ordering quirks:
// yolo precedes disallowedTools; auto always denies.
type CollaborationMode string

const (
	ModePlan  CollaborationMode = "plan"
	ModeBuild CollaborationMode = "build"
	ModeEdit  CollaborationMode = "edit"
	ModeYolo  CollaborationMode = "yolo"
	ModeAuto  CollaborationMode = "auto"
)

// PermAction is what the pipeline decided.
type PermAction string

const (
	PermActionAllow PermAction = "allow"
	PermActionDeny  PermAction = "deny"
	PermActionAsk   PermAction = "ask"
)

// PermissionDecisionOut is the pipeline result.
type PermissionDecisionOut struct {
	Action        PermAction
	Message       string
	AlwaysAskFlag bool // ask originating from alwaysAsk is un-overridable by hooks
}

// PermissionRule mirrors {toolName, ruleContent?}.
type PermissionRule struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// Ruleset is the project/session ruleset.
type Ruleset struct {
	Allow []PermissionRule `json:"allow,omitempty"`
	Deny  []PermissionRule `json:"deny,omitempty"`
	Ask   []PermissionRule `json:"ask,omitempty"`
}

// SessionContext carries per-session permission state.
type SessionContext struct {
	SessionID    string
	WorkspaceDir string
	Mode         CollaborationMode
	PlanEnabled  bool
	Rules        Ruleset           // project rules (persisted)
	SessionAllow []PermissionRule  // session-scoped "always allow", dies with session
	Disallowed   []string          // config disallowedTools
}

// Broker is the UI ask surface.
type Broker interface {
	Ask(ctx context.Context, session *SessionContext, tool string, input map[string]any, decision PermissionDecisionOut) (granted bool, message string)
}

// PermissionService port of checkPermission (§4.2). Ordering is a behavioral
// contract; do not "clean it up".
type PermissionService struct {
	Broker Broker
}

func (p *PermissionService) Check(ctx context.Context, s *SessionContext, e *ToolEntry, toolName string, input map[string]any) (PermissionDecisionOut, error) {
	m := e.Metadata

	// 1. Plan-mode transitions
	if toolName == "EnterPlanMode" {
		return PermissionDecisionOut{Action: PermActionAllow}, nil
	}
	if toolName == "ExitPlanMode" && !s.PlanEnabled {
		return PermissionDecisionOut{Action: PermActionDeny, Message: "plan mode not active"}, nil
	}

	// 2. requiresUserInteraction
	if m.RequiresUserInteraction {
		if containsString(s.Disallowed, toolName) {
			return PermissionDecisionOut{Action: PermActionDeny, Message: "tool is disallowed"}, nil
		}
		return PermissionDecisionOut{Action: PermActionAsk}, nil
	}

	// 3. alwaysAsk capability
	if m.NeedsApproval {
		if s.Mode == ModeAuto {
			return PermissionDecisionOut{Action: PermActionDeny, Message: "mode.auto.unimplemented"}, nil
		}
		if containsString(s.Disallowed, toolName) {
			return PermissionDecisionOut{Action: PermActionDeny, Message: "tool is disallowed"}, nil
		}
		if r, ok := matchRules(s.Rules.Deny, toolName, input); ok {
			_ = r
			return PermissionDecisionOut{Action: PermActionDeny, Message: "denied by project rule"}, nil
		}
		if r, ok := matchRules(s.SessionAllow, toolName, input); ok {
			_ = r
			return PermissionDecisionOut{Action: PermActionAllow}, nil // rule.session.allow
		}
		return PermissionDecisionOut{Action: PermActionAsk, AlwaysAskFlag: true}, nil
	}

	// 4. yolo allows (before disallowedTools — deliberate)
	if s.Mode == ModeYolo {
		return PermissionDecisionOut{Action: PermActionAllow}, nil
	}

	// 5. auto always denies
	if s.Mode == ModeAuto {
		return PermissionDecisionOut{Action: PermActionDeny, Message: "mode.auto.unimplemented"}, nil
	}

	// 6. config disallowedTools
	if containsString(s.Disallowed, toolName) {
		return PermissionDecisionOut{Action: PermActionDeny, Message: "tool is disallowed"}, nil
	}

	// 7. project rules deny → deny; ask → ask
	if _, ok := matchRules(s.Rules.Deny, toolName, input); ok {
		return PermissionDecisionOut{Action: PermActionDeny, Message: "denied by project rule"}, nil
	}
	if _, ok := matchRules(s.Rules.Ask, toolName, input); ok {
		return PermissionDecisionOut{Action: PermActionAsk}, nil
	}

	// 8. plan mode: read-only and non-destructive only
	if s.PlanEnabled {
		if (m.ReadOnly || ReadOnlySet[toolName]) && !m.Destructive {
			return PermissionDecisionOut{Action: PermActionAllow}, nil
		}
		if e.ProviderID != "" && !m.Destructive {
			return PermissionDecisionOut{Action: PermActionAllow}, nil // mode.plan.mcp
		}
		return PermissionDecisionOut{Action: PermActionDeny, Message: "mode.plan.nonReadOnly"}, nil
	}

	// 9. project allow
	if _, ok := matchRules(s.Rules.Allow, toolName, input); ok {
		return PermissionDecisionOut{Action: PermActionAllow}, nil
	}

	// 10. mode checks
	if s.Mode == ModeEdit {
		return p.checkEditMode(s, m, toolName)
	}
	return p.checkBuildMode(s, m, toolName)
}

func (p *PermissionService) checkBuildMode(s *SessionContext, m ToolMetadata, toolName string) (PermissionDecisionOut, error) {
	if (m.ReadOnly || ReadOnlySet[toolName]) && !m.Destructive && !m.NeedsApproval {
		return PermissionDecisionOut{Action: PermActionAllow}, nil
	}
	if m.NeedsApproval || m.Destructive || (m.SideEffectScope != "" && m.SideEffectScope != "none") {
		return PermissionDecisionOut{Action: PermActionAsk}, nil
	}
	return PermissionDecisionOut{Action: PermActionAllow}, nil
}

func (p *PermissionService) checkEditMode(s *SessionContext, m ToolMetadata, toolName string) (PermissionDecisionOut, error) {
	// Edit mode: edit-class tools (Write/Edit/NotebookEdit family) are allowed;
	// everything else follows build rules.
	switch toolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return PermissionDecisionOut{Action: PermActionAllow}, nil
	}
	return p.checkBuildMode(s, m, toolName)
}

// matchRules port of rule-matching.ts: tool-name match (exact; Write matches
// an Edit rule), subject = first present of command,url,file_path,path,
// pattern,patch_text; content match prefix:*/wildcard/exact.
func matchRules(rules []PermissionRule, toolName string, input map[string]any) (PermissionRule, bool) {
	for _, r := range rules {
		if !toolNameMatches(r.ToolName, toolName) {
			continue
		}
		if r.RuleContent == "" {
			return r, true
		}
		subject := subjectOf(input)
		if subject == "" {
			continue
		}
		if contentMatches(r.RuleContent, subject) {
			return r, true
		}
	}
	return PermissionRule{}, false
}

func toolNameMatches(ruleName, toolName string) bool {
	if ruleName == toolName {
		return true
	}
	// Write matches an Edit rule
	if toolName == "Write" && ruleName == "Edit" {
		return true
	}
	return false
}

func subjectOf(input map[string]any) string {
	for _, k := range []string{"command", "url", "file_path", "path", "pattern", "patch_text"} {
		if v, ok := input[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func contentMatches(rule, subject string) bool {
	if strings.HasSuffix(rule, ":*") {
		prefix := strings.TrimSuffix(rule, ":*")
		return subject == prefix || strings.HasPrefix(subject, prefix+" ") || strings.HasPrefix(subject, prefix+"\t")
	}
	if strings.Contains(rule, "*") {
		pattern := regexp.QuoteMeta(rule)
		pattern = strings.ReplaceAll(pattern, `\*`, ".*")
		ok := regexp.MustCompile("^" + pattern + "$").MatchString(subject)
		return ok
	}
	return rule == subject
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// DefaultBroker denies everything (matches source default when no UI).
type DefaultBroker struct{}

func (DefaultBroker) Ask(_ context.Context, _ *SessionContext, tool string, _ map[string]any, _ PermissionDecisionOut) (bool, string) {
	return false, fmt.Sprintf("permission to use %s requires user approval and no broker is attached", tool)
}
