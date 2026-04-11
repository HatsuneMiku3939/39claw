package app

import "strings"

type ParsedTaskOverride struct {
	Matched       bool
	TaskName      string
	Prompt        string
	RejectMessage string
}

func ParseTaskOverride(content string, hasImages bool) ParsedTaskOverride {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "task:") {
		return ParsedTaskOverride{Prompt: trimmed}
	}

	remainder := strings.TrimPrefix(trimmed, "task:")
	if remainder == "" {
		return ParsedTaskOverride{
			RejectMessage: "Invalid task override. Use `task:<name>` with a slug-style task name such as `task:release-bot-v1`.",
		}
	}

	nameEnd := 0
	for nameEnd < len(remainder) {
		switch remainder[nameEnd] {
		case ' ', '\t', '\n', '\r':
			goto parsedName
		default:
			nameEnd++
		}
	}

parsedName:
	taskName := remainder[:nameEnd]
	if err := ValidateTaskName(taskName); err != nil {
		return ParsedTaskOverride{
			RejectMessage: "Invalid task override. Use `task:<name>` with a slug-style task name such as `task:release-bot-v1`.",
		}
	}

	prompt := strings.TrimSpace(remainder[nameEnd:])
	if prompt == "" && !hasImages {
		return ParsedTaskOverride{
			RejectMessage: "Task override messages need body text or at least one image attachment.",
		}
	}

	return ParsedTaskOverride{
		Matched:  true,
		TaskName: taskName,
		Prompt:   prompt,
	}
}
