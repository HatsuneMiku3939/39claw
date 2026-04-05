package discord

import "testing"

func TestFormatDiscordResponseTextRewritesWorkspaceMarkdownLinks(t *testing.T) {
	t.Parallel()

	workdir := "/home/filepang/Documents/filepang"
	text := "그리고 [디펜더봇 평가-머지-릴리즈 자동화](" +
		"/home/filepang/Documents/filepang/1%20Project/direnv-action/%EB%94%94%ED%8E%9C%EB%8D%94%EB%B4%87%20%ED%8F%89%EA%B0%80-%EB%A8%B8%EC%A7%80-%EB%A6%B4%EB%A6%AC%EC%A6%88%20%EC%9E%90%EB%8F%99%ED%99%94.md" +
		") 노트도 같이 업데이트했어."

	got := formatDiscordResponseText(text, workdir)
	want := "그리고 디펜더봇 평가-머지-릴리즈 자동화 (`workspace/1 Project/direnv-action/디펜더봇 평가-머지-릴리즈 자동화.md`) 노트도 같이 업데이트했어."
	if got != want {
		t.Fatalf("formatDiscordResponseText() = %q, want %q", got, want)
	}
}

func TestFormatDiscordResponseTextRewritesBareWorkspacePaths(t *testing.T) {
	t.Parallel()

	workdir := "/home/filepang/Documents/filepang"
	text := "여기 경로를 확인해: /home/filepang/Documents/filepang/1%20Project/%ED%95%9C%EA%B8%80.md"

	got := formatDiscordResponseText(text, workdir)
	want := "여기 경로를 확인해: workspace/1 Project/한글.md"
	if got != want {
		t.Fatalf("formatDiscordResponseText() = %q, want %q", got, want)
	}
}
