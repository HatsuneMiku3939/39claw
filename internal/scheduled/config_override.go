package scheduled

import "fmt"

const mcpServerName = "scheduled-tasks"

func BuildMCPURLConfigOverride(serverURL string) string {
	return fmt.Sprintf(`mcp_servers.%s={url = %q}`, mcpServerName, serverURL)
}
