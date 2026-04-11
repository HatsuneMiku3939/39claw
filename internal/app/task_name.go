package app

import (
	"errors"
	"regexp"
	"strings"
)

const TaskNameRulesDescription = "Task names must use lowercase letters, numbers, or single hyphens, start with a letter, end with a letter or number, and be 3 to 32 characters long."

var taskNamePattern = regexp.MustCompile(`^[a-z](?:[a-z0-9-]{1,30}[a-z0-9])?$`)

func ValidateTaskName(taskName string) error {
	name := strings.TrimSpace(taskName)
	if name == "" {
		return errors.New("task name must not be empty")
	}

	if strings.Contains(name, "--") {
		return errors.New("invalid task name")
	}

	if !taskNamePattern.MatchString(name) {
		return errors.New("invalid task name")
	}

	return nil
}
