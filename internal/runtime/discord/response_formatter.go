package discord

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var markdownLinkPattern = regexp.MustCompile(`\[(?P<label>[^\]]+)\]\((?P<target>[^)\n]+)\)`)

func formatDiscordResponseText(text string, workdir string) string {
	text = rewriteMarkdownLinks(text, workdir)
	text = rewriteWorkspacePaths(text, workdir)
	return text
}

func rewriteMarkdownLinks(text string, workdir string) string {
	matches := markdownLinkPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var builder strings.Builder
	last := 0
	for _, match := range matches {
		start := match[0]
		end := match[1]
		if start > 0 && text[start-1] == '!' {
			continue
		}

		builder.WriteString(text[last:start])

		label := strings.TrimSpace(text[match[2]:match[3]])
		target := strings.TrimSpace(text[match[4]:match[5]])
		displayTarget := formatDiscordLinkTarget(target, workdir)

		switch {
		case label == "" && displayTarget == "":
		case displayTarget == "":
			builder.WriteString(label)
		case label == "" || label == displayTarget:
			builder.WriteString("`")
			builder.WriteString(displayTarget)
			builder.WriteString("`")
		default:
			builder.WriteString(label)
			builder.WriteString(" (`")
			builder.WriteString(displayTarget)
			builder.WriteString("`)")
		}

		last = end
	}

	builder.WriteString(text[last:])
	return builder.String()
}

func formatDiscordLinkTarget(target string, workdir string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}

	return sanitizeLocalPathForDiscord(target, workdir)
}

func sanitizeLocalPathForDiscord(rawPath string, workdir string) string {
	pathPart, fragment := splitFragment(rawPath)
	pathPart = decodePercentEscapes(pathPart)

	if workdir != "" {
		cleanWorkdir := filepath.Clean(workdir)
		cleanPath := filepath.Clean(pathPart)
		if relativePath, ok := workspaceRelativePath(cleanWorkdir, cleanPath); ok {
			pathPart = relativePath
		} else {
			pathPart = filepath.ToSlash(cleanPath)
		}
	} else {
		pathPart = filepath.ToSlash(filepath.Clean(pathPart))
	}

	if fragment == "" {
		return pathPart
	}

	return pathPart + "#" + decodePercentEscapes(fragment)
}

func workspaceRelativePath(workdir string, targetPath string) (string, bool) {
	relativePath, err := filepath.Rel(workdir, targetPath)
	if err != nil {
		return "", false
	}

	if relativePath == "." {
		return "workspace", true
	}

	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", false
	}

	return filepath.ToSlash(filepath.Join("workspace", relativePath)), true
}

func rewriteWorkspacePaths(text string, workdir string) string {
	if strings.TrimSpace(workdir) == "" {
		return text
	}

	text = strings.ReplaceAll(text, workdir, "workspace")

	var builder strings.Builder
	for index := 0; index < len(text); {
		if !strings.HasPrefix(text[index:], "workspace") {
			builder.WriteByte(text[index])
			index++
			continue
		}

		end := index + len("workspace")
		for end < len(text) && isWorkspacePathRune(text[end]) {
			end++
		}

		builder.WriteString(decodePercentEscapes(text[index:end]))
		index = end
	}

	return builder.String()
}

func isWorkspacePathRune(value byte) bool {
	switch value {
	case '/', '#', '.', '-', '_', '~', '%':
		return true
	}

	if value >= '0' && value <= '9' {
		return true
	}

	if value >= 'A' && value <= 'Z' {
		return true
	}

	if value >= 'a' && value <= 'z' {
		return true
	}

	return false
}

func decodePercentEscapes(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}

	return decoded
}

func splitFragment(value string) (string, string) {
	pathPart, fragment, found := strings.Cut(value, "#")
	if !found {
		return value, ""
	}

	return pathPart, fragment
}
