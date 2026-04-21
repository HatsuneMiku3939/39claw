package codex

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const defaultExecutablePath = "codex"
const originatorEnvKey = "CODEX_INTERNAL_ORIGINATOR_OVERRIDE"
const originatorValue = "codex_sdk_go"

type executor struct {
	executablePath string
	env            map[string]string
	baseURL        string
	apiKey         string
}

type execRequest struct {
	prompt        string
	images        []string
	threadID      string
	threadOptions ThreadOptions
}

func newExecutor(options Options) *executor {
	executablePath := options.ExecutablePath
	if executablePath == "" {
		executablePath = defaultExecutablePath
	}

	return &executor{
		executablePath: executablePath,
		env:            cloneStringMap(options.Env),
		baseURL:        options.BaseURL,
		apiKey:         options.APIKey,
	}
}

func (e *executor) run(ctx context.Context, request execRequest, handleLine func(string) error) error {
	commandArgs := e.buildArgs(request)
	//nolint:gosec // The executable path is intentionally configurable for tests and local Codex installations.
	cmd := exec.CommandContext(ctx, e.executablePath, commandArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create codex stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(request.prompt)
	cmd.Env = environFromMap(e.buildEnv())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start codex exec: %w", err)
	}

	reader := bufio.NewReader(stdout)
	for {
		line, readErr := readJSONLLine(reader)
		if readErr != nil {
			if readErr == io.EOF {
				break
			}

			shutdownProcess(cmd)

			return fmt.Errorf("read codex stream: %w", readErr)
		}

		if line == "" {
			continue
		}

		if err := handleLine(line); err != nil {
			shutdownProcess(cmd)

			return err
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return fmt.Errorf("codex exec exited: %w", err)
		}

		return fmt.Errorf("codex exec exited: %w: %s", err, message)
	}

	return nil
}

func (e *executor) buildArgs(request execRequest) []string {
	args := []string{"exec", "--experimental-json"}

	if e.baseURL != "" {
		args = append(args, "--config", "openai_base_url="+strconv.Quote(e.baseURL))
	}

	for _, override := range request.threadOptions.ConfigOverrides {
		if strings.TrimSpace(override) == "" {
			continue
		}
		args = append(args, "--config", override)
	}

	if request.threadOptions.Model != "" {
		args = append(args, "--model", request.threadOptions.Model)
	}

	if request.threadOptions.SandboxMode != "" {
		args = append(args, "--sandbox", string(request.threadOptions.SandboxMode))
	}

	if request.threadOptions.WorkingDirectory != "" {
		args = append(args, "--cd", request.threadOptions.WorkingDirectory)
	}

	for _, dir := range request.threadOptions.AdditionalDirectories {
		args = append(args, "--add-dir", dir)
	}

	if request.threadOptions.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}

	if request.threadOptions.ModelReasoningEffort != "" {
		args = append(args, "--config", "model_reasoning_effort="+strconv.Quote(string(request.threadOptions.ModelReasoningEffort)))
	}

	if request.threadOptions.NetworkAccessEnabled != nil {
		args = append(args, "--config", fmt.Sprintf("sandbox_workspace_write.network_access=%t", *request.threadOptions.NetworkAccessEnabled))
	}

	if request.threadOptions.WebSearchMode != "" {
		args = append(args, "--config", "web_search="+strconv.Quote(string(request.threadOptions.WebSearchMode)))
	}

	if request.threadOptions.ApprovalPolicy != "" {
		args = append(args, "--config", "approval_policy="+strconv.Quote(string(request.threadOptions.ApprovalPolicy)))
	}

	if request.threadID != "" {
		args = append(args, "resume", request.threadID)
	}

	for _, image := range request.images {
		args = append(args, "--image", image)
	}

	return args
}

func (e *executor) buildEnv() map[string]string {
	env := cloneCurrentEnv()

	for key, value := range e.env {
		env[key] = value
	}

	if env[originatorEnvKey] == "" {
		env[originatorEnvKey] = originatorValue
	}

	if e.apiKey != "" {
		env["CODEX_API_KEY"] = e.apiKey
	}

	return env
}

func cloneCurrentEnv() map[string]string {
	env := make(map[string]string, len(os.Environ()))

	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		env[key] = value
	}

	return env
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

func environFromMap(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}

	env := make([]string, 0, len(values))
	for key, value := range values {
		env = append(env, key+"="+value)
	}

	return env
}

func readJSONLLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	if err == io.EOF && line == "" {
		return "", io.EOF
	}

	return strings.TrimRight(line, "\r\n"), nil
}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}

func shutdownProcess(cmd *exec.Cmd) {
	killErr := killProcess(cmd)
	if killErr != nil && !strings.Contains(killErr.Error(), "process already finished") {
		return
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		return
	}
}
