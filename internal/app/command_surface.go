package app

import "fmt"

type commandSurface struct {
	commandName string
}

func newCommandSurface(commandName string) commandSurface {
	return commandSurface{commandName: commandName}
}

func (s commandSurface) taskList() string {
	return fmt.Sprintf("`/%s action:task-list`", s.commandName)
}

func (s commandSurface) taskNewPlaceholder() string {
	return fmt.Sprintf("`/%s action:task-new task_name:<name>`", s.commandName)
}

func (s commandSurface) taskSwitchPlaceholder() string {
	return fmt.Sprintf("`/%s action:task-switch task_name:<name>`", s.commandName)
}

func (s commandSurface) taskSwitch(taskName string) string {
	return fmt.Sprintf("`/%s action:task-switch task_name:%s`", s.commandName, taskName)
}

func (s commandSurface) taskClose(taskID string) string {
	return fmt.Sprintf("`/%s action:task-close task_id:%s`", s.commandName, taskID)
}

func (s commandSurface) taskResetContext() string {
	return fmt.Sprintf("`/%s action:task-reset-context`", s.commandName)
}

func (s commandSurface) taskIDPlaceholder(action string) string {
	return fmt.Sprintf("`/%s action:%s task_id:<id>`", s.commandName, action)
}
