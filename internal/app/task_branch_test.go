package app_test

import (
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestDefaultTaskBranchName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		taskName string
		taskID   string
		want     string
	}{
		{
			name:     "ascii words become hyphenated branch names",
			taskName: "Release work",
			taskID:   "task-1",
			want:     "task/release-work",
		},
		{
			name:     "special characters collapse into separators",
			taskName: "Fix/API v2 !!!",
			taskID:   "task-2",
			want:     "task/fix-api-v2",
		},
		{
			name:     "unicode letters are preserved",
			taskName: "릴리즈 준비",
			taskID:   "task-3",
			want:     "task/릴리즈-준비",
		},
		{
			name:     "empty normalized names fall back to task id",
			taskName: "!!!",
			taskID:   "task-4",
			want:     "task/task-4",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := app.DefaultTaskBranchName(testCase.taskName, testCase.taskID); got != testCase.want {
				t.Fatalf("DefaultTaskBranchName() = %q, want %q", got, testCase.want)
			}
		})
	}
}
