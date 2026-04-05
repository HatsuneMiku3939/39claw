package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	helperWrapperPath string
	helperWrapperErr  error
	helperWrapperOnce sync.Once
)

func TestThreadRunReturnsCompletedTurn(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, "single-success")
	thread := client.StartThread(ThreadOptions{})

	turn, err := thread.Run(context.Background(), TextInput("hello"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if thread.ID() != "thread_test" {
		t.Fatalf("thread ID = %q, want %q", thread.ID(), "thread_test")
	}

	if turn.FinalResponse != "Hi from helper" {
		t.Fatalf("final response = %q, want %q", turn.FinalResponse, "Hi from helper")
	}

	if len(turn.Items) != 1 {
		t.Fatalf("items length = %d, want %d", len(turn.Items), 1)
	}

	if turn.Usage == nil {
		t.Fatal("usage is nil")
	}

	if turn.Usage.OutputTokens != 5 {
		t.Fatalf("output tokens = %d, want %d", turn.Usage.OutputTokens, 5)
	}
}

func TestThreadRunStreamedEmitsEvents(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, "single-success")
	thread := client.StartThread(ThreadOptions{})

	stream, err := thread.RunStreamed(context.Background(), TextInput("hello"))
	if err != nil {
		t.Fatalf("RunStreamed() error = %v", err)
	}

	var events []Event
	for event := range stream.Events() {
		events = append(events, event)
	}

	if err := stream.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("event count = %d, want %d", len(events), 4)
	}

	if events[0].Type != "thread.started" || events[0].ThreadID != "thread_test" {
		t.Fatalf("first event = %#v", events[0])
	}

	if events[2].Item == nil || events[2].Item.Type != "agent_message" || events[2].Item.Text != "Hi from helper" {
		t.Fatalf("third event item = %#v", events[2].Item)
	}
}

func TestThreadRunPassesMultipartInputAndOptions(t *testing.T) {
	t.Parallel()

	networkAccessEnabled := true
	client, recorder := newTestClient(t, "single-success")
	thread := client.StartThread(ThreadOptions{
		Model:                 "gpt-test",
		SandboxMode:           SandboxModeWorkspaceWrite,
		WorkingDirectory:      "/workspace/project",
		AdditionalDirectories: []string{"/workspace/shared"},
		SkipGitRepoCheck:      true,
		ModelReasoningEffort:  ModelReasoningEffortHigh,
		NetworkAccessEnabled:  &networkAccessEnabled,
		WebSearchMode:         WebSearchModeLive,
		ApprovalPolicy:        ApprovalModeOnRequest,
	})

	_, err := thread.Run(context.Background(), MultiPartInput(
		TextPart("Describe this image"),
		LocalImagePart("/tmp/example.png"),
		TextPart("Keep it short"),
	))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	record := readInvocationRecord(t, recorder)

	if record.Stdin != "Describe this image\n\nKeep it short" {
		t.Fatalf("stdin = %q", record.Stdin)
	}

	expectedPairs := [][]string{
		{"--model", "gpt-test"},
		{"--sandbox", "workspace-write"},
		{"--cd", "/workspace/project"},
		{"--add-dir", "/workspace/shared"},
		{"--config", `model_reasoning_effort="high"`},
		{"--config", "sandbox_workspace_write.network_access=true"},
		{"--config", `web_search="live"`},
		{"--config", `approval_policy="on-request"`},
		{"--image", "/tmp/example.png"},
	}

	for _, pair := range expectedPairs {
		if !hasArgPair(record.Args, pair[0], pair[1]) {
			t.Fatalf("args missing pair %v: %v", pair, record.Args)
		}
	}

	if !hasArg(record.Args, "--skip-git-repo-check") {
		t.Fatalf("args missing --skip-git-repo-check: %v", record.Args)
	}
}

func TestResumeThreadPassesResumeArguments(t *testing.T) {
	t.Parallel()

	client, recorder := newTestClient(t, "resume-success")
	thread := client.ResumeThread("thread_saved", ThreadOptions{})

	if _, err := thread.Run(context.Background(), TextInput("resume me")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	record := readInvocationRecord(t, recorder)
	if !hasArgPair(record.Args, "resume", "thread_saved") {
		t.Fatalf("args missing resume pair: %v", record.Args)
	}

	if thread.ID() != "thread_saved" {
		t.Fatalf("thread ID = %q, want %q", thread.ID(), "thread_saved")
	}
}

func TestRunReturnsTurnFailure(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, "turn-failed")
	thread := client.StartThread(ThreadOptions{})

	_, err := thread.Run(context.Background(), TextInput("fail"))
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}

	if err.Error() != "helper turn failed" {
		t.Fatalf("error = %q, want %q", err.Error(), "helper turn failed")
	}
}

func TestRunStreamedReturnsProcessFailure(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, "process-failed")
	thread := client.StartThread(ThreadOptions{})

	stream, err := thread.RunStreamed(context.Background(), TextInput("explode"))
	if err != nil {
		t.Fatalf("RunStreamed() error = %v", err)
	}

	for range stream.Events() {
	}

	err = stream.Wait()
	if err == nil {
		t.Fatal("Wait() error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), "helper process failed") {
		t.Fatalf("error = %q, want stderr detail", err.Error())
	}
}

func TestInputNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   Input
		want    normalizedInput
		wantErr string
	}{
		{
			name:  "text input joins prompt",
			input: TextInput("hello"),
			want: normalizedInput{
				Prompt: "hello",
			},
		},
		{
			name: "multipart input splits images",
			input: MultiPartInput(
				TextPart("alpha"),
				LocalImagePart("/tmp/one.png"),
				TextPart("beta"),
				LocalImagePart("/tmp/two.png"),
			),
			want: normalizedInput{
				Prompt: "alpha\n\nbeta",
				Images: []string{"/tmp/one.png", "/tmp/two.png"},
			},
		},
		{
			name: "missing image path fails",
			input: Input{
				Parts: []InputPart{{Type: InputPartTypeLocalImage}},
			},
			wantErr: "local image input requires a path",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.input.normalize()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("normalize() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("normalize() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("normalize() error = %v", err)
			}

			if got.Prompt != tt.want.Prompt {
				t.Fatalf("prompt = %q, want %q", got.Prompt, tt.want.Prompt)
			}

			if strings.Join(got.Images, ",") != strings.Join(tt.want.Images, ",") {
				t.Fatalf("images = %v, want %v", got.Images, tt.want.Images)
			}
		})
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CODEX_TEST_HELPER") != "1" {
		return
	}

	if err := runHelperProcess(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(0)
}

type helperInvocation struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin"`
}

func newTestClient(t *testing.T, scenario string) (*Client, string) {
	t.Helper()

	recorder := filepath.Join(t.TempDir(), "invocations.jsonl")
	wrapper := createHelperWrapper(t)

	client := New(Options{
		ExecutablePath: wrapper,
		Env: map[string]string{
			"GO_WANT_CODEX_TEST_HELPER": "1",
			"CODEX_TEST_SCENARIO":       scenario,
			"CODEX_TEST_RECORD_FILE":    recorder,
		},
	})

	return client, recorder
}

func createHelperWrapper(t *testing.T) string {
	t.Helper()

	helperWrapperOnce.Do(func() {
		executable, err := os.Executable()
		if err != nil {
			helperWrapperErr = fmt.Errorf("os.Executable() error: %w", err)
			return
		}

		tempDir, err := os.MkdirTemp("", "codex-helper-*")
		if err != nil {
			helperWrapperErr = fmt.Errorf("os.MkdirTemp() error: %w", err)
			return
		}

		wrapperPath := filepath.Join(tempDir, "codex-helper")
		script := fmt.Sprintf("#!/usr/bin/env bash\nexec %q -test.run=TestHelperProcess -- \"$@\"\n", executable)

		if err := os.WriteFile(wrapperPath, []byte(script), 0o755); err != nil {
			helperWrapperErr = fmt.Errorf("os.WriteFile() error: %w", err)
			return
		}

		helperWrapperPath = wrapperPath
	})

	if helperWrapperErr != nil {
		t.Fatal(helperWrapperErr)
	}

	return helperWrapperPath
}

func runHelperProcess() error {
	args := helperArgs(os.Args)
	stdin, err := ioReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	recordFile := os.Getenv("CODEX_TEST_RECORD_FILE")
	if err := appendInvocationRecord(recordFile, helperInvocation{
		Args:  args,
		Stdin: string(stdin),
	}); err != nil {
		return fmt.Errorf("append record: %w", err)
	}

	switch os.Getenv("CODEX_TEST_SCENARIO") {
	case "single-success":
		writeHelperEvent(map[string]any{"type": "thread.started", "thread_id": "thread_test"})
		writeHelperEvent(map[string]any{"type": "turn.started"})
		writeHelperEvent(map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"id":   "item_1",
				"type": "agent_message",
				"text": "Hi from helper",
			},
		})
		writeHelperEvent(map[string]any{
			"type": "turn.completed",
			"usage": map[string]any{
				"input_tokens":        42,
				"cached_input_tokens": 12,
				"output_tokens":       5,
			},
		})
		return nil
	case "resume-success":
		writeHelperEvent(map[string]any{"type": "turn.started"})
		writeHelperEvent(map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"id":   "item_2",
				"type": "agent_message",
				"text": "Resumed helper response",
			},
		})
		writeHelperEvent(map[string]any{
			"type": "turn.completed",
			"usage": map[string]any{
				"input_tokens":        10,
				"cached_input_tokens": 0,
				"output_tokens":       3,
			},
		})
		return nil
	case "turn-failed":
		writeHelperEvent(map[string]any{"type": "thread.started", "thread_id": "thread_test"})
		writeHelperEvent(map[string]any{"type": "turn.started"})
		writeHelperEvent(map[string]any{
			"type": "turn.failed",
			"error": map[string]any{
				"message": "helper turn failed",
			},
		})
		return nil
	case "process-failed":
		_, _ = fmt.Fprintln(os.Stderr, "helper process failed")
		return errors.New("helper process failed")
	case "stream-progress":
		writeHelperEvent(map[string]any{"type": "thread.started", "thread_id": "thread_stream"})
		writeHelperEvent(map[string]any{"type": "turn.started"})
		writeHelperEvent(map[string]any{
			"type": "item.started",
			"item": map[string]any{
				"id":      "item_cmd",
				"type":    "command_execution",
				"command": "go test ./...",
				"status":  "in_progress",
			},
		})
		writeHelperEvent(map[string]any{
			"type": "item.updated",
			"item": map[string]any{
				"id":   "item_msg",
				"type": "agent_message",
				"text": "Partial helper response",
			},
		})
		writeHelperEvent(map[string]any{
			"type": "item.completed",
			"item": map[string]any{
				"id":   "item_msg",
				"type": "agent_message",
				"text": "Final helper response",
			},
		})
		writeHelperEvent(map[string]any{
			"type": "turn.completed",
			"usage": map[string]any{
				"input_tokens":        8,
				"cached_input_tokens": 0,
				"output_tokens":       4,
			},
		})
		return nil
	default:
		return fmt.Errorf("unknown helper scenario %q", os.Getenv("CODEX_TEST_SCENARIO"))
	}
}

func helperArgs(args []string) []string {
	for index, arg := range args {
		if arg == "--" {
			return append([]string(nil), args[index+1:]...)
		}
	}

	return nil
}

func appendInvocationRecord(path string, record helperInvocation) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return err
	}

	return nil
}

func readInvocationRecord(t *testing.T, path string) helperInvocation {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(content), []byte{'\n'})
	if len(lines) == 0 || len(lines[0]) == 0 {
		t.Fatal("no helper invocations recorded")
	}

	var record helperInvocation
	if err := json.Unmarshal(lines[0], &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	return record
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func hasArgPair(args []string, key string, value string) bool {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}

	return false
}

func writeHelperEvent(event map[string]any) {
	payload, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(payload))
}

func ioReadAll(file *os.File) ([]byte, error) {
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(file); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
