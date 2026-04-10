package app

import (
	"strings"
	"unicode"
)

func DefaultTaskBranchName(taskName string, taskID string) string {
	branchSuffix := taskBranchSlug(taskName)
	if branchSuffix == "" {
		branchSuffix = strings.TrimSpace(taskID)
	}
	if branchSuffix == "" {
		branchSuffix = "unnamed-task"
	}

	return "task/" + branchSuffix
}

func taskBranchSlug(taskName string) string {
	normalized := strings.ToLower(strings.TrimSpace(taskName))
	if normalized == "" {
		return ""
	}

	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range normalized {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			builder.WriteRune(r)
			lastWasSeparator = false
		default:
			if builder.Len() == 0 || lastWasSeparator {
				continue
			}
			builder.WriteByte('-')
			lastWasSeparator = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
