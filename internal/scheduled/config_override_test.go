package scheduled

import (
	"strings"
	"testing"
)

func TestBuildMCPConfigOverride(t *testing.T) {
	t.Parallel()

	override := BuildMCPConfigOverride(
		"/usr/local/bin/39claw",
		"/tmp/39claw.sqlite",
		"Asia/Tokyo",
		"123456",
	)

	for _, want := range []string{
		`mcp_servers.scheduled-tasks={`,
		`command = "/usr/local/bin/39claw"`,
		`"mcp-scheduled-tasks"`,
		`"--sqlite-path"`,
		`"/tmp/39claw.sqlite"`,
		`"--timezone"`,
		`"Asia/Tokyo"`,
		`"--default-report-channel-id"`,
		`"123456"`,
	} {
		if !strings.Contains(override, want) {
			t.Fatalf("override missing %q:\n%s", want, override)
		}
	}
}
