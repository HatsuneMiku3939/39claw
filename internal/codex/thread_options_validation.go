package codex

import "fmt"

func ParseSandboxMode(value string) (SandboxMode, error) {
	switch SandboxMode(value) {
	case SandboxModeReadOnly, SandboxModeWorkspaceWrite, SandboxModeDangerFullAccess:
		return SandboxMode(value), nil
	default:
		return "", fmt.Errorf("invalid sandbox mode %q", value)
	}
}

func ParseApprovalPolicy(value string) (ApprovalMode, error) {
	switch ApprovalMode(value) {
	case ApprovalModeNever, ApprovalModeOnRequest, ApprovalModeOnFailure, ApprovalModeUntrusted:
		return ApprovalMode(value), nil
	default:
		return "", fmt.Errorf("invalid approval policy %q", value)
	}
}

func ParseModelReasoningEffort(value string) (ModelReasoningEffort, error) {
	switch ModelReasoningEffort(value) {
	case ModelReasoningEffortMinimal,
		ModelReasoningEffortLow,
		ModelReasoningEffortMedium,
		ModelReasoningEffortHigh,
		ModelReasoningEffortXHigh:
		return ModelReasoningEffort(value), nil
	default:
		return "", fmt.Errorf("invalid model reasoning effort %q", value)
	}
}

func ParseWebSearchMode(value string) (WebSearchMode, error) {
	switch WebSearchMode(value) {
	case WebSearchModeDisabled, WebSearchModeCached, WebSearchModeLive:
		return WebSearchMode(value), nil
	default:
		return "", fmt.Errorf("invalid web search mode %q", value)
	}
}
