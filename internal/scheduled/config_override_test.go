package scheduled

import (
	"strings"
	"testing"
)

func TestBuildMCPURLConfigOverride(t *testing.T) {
	t.Parallel()

	override := BuildMCPURLConfigOverride("http://127.0.0.1:4000/mcp/scheduled-tasks/sse")

	for _, want := range []string{
		`mcp_servers.scheduled-tasks={`,
		`url = "http://127.0.0.1:4000/mcp/scheduled-tasks/sse"`,
	} {
		if !strings.Contains(override, want) {
			t.Fatalf("override missing %q:\n%s", want, override)
		}
	}
}
