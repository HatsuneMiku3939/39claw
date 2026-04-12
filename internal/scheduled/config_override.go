package scheduled

import (
	"fmt"
	"strings"
)

const mcpServerName = "scheduled-tasks"

func BuildMCPConfigOverride(
	serverExecutablePath string,
	sqlitePath string,
	timezoneName string,
	defaultReportChannelID string,
) string {
	args := []string{
		"mcp-scheduled-tasks",
		"--sqlite-path",
		sqlitePath,
		"--timezone",
		timezoneName,
	}
	if strings.TrimSpace(defaultReportChannelID) != "" {
		args = append(args, "--default-report-channel-id", defaultReportChannelID)
	}

	quotedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}

	return fmt.Sprintf(
		`mcp_servers.%s={command = %q, args = [%s]}`,
		mcpServerName,
		serverExecutablePath,
		strings.Join(quotedArgs, ", "),
	)
}
