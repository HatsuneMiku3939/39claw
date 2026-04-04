package discord

import (
	"strings"
)

const discordMessageLimit = 2000

func chunkText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	lines := strings.SplitAfter(text, "\n")
	chunks := make([]string, 0, 1)
	var current strings.Builder
	openFence := ""

	flush := func() {
		if current.Len() == 0 {
			return
		}

		chunk := strings.TrimSpace(current.String())
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		current.Reset()
	}

	appendWithFence := func(line string) {
		line = strings.ReplaceAll(line, "\r\n", "\n")
		if line == "" {
			return
		}

		for len(line) > 0 {
			prefix := ""
			suffix := ""
			if current.Len() == 0 && openFence != "" {
				prefix = openFence + "\n"
			}
			if openFence != "" {
				suffix = "\n```"
			}

			available := discordMessageLimit - len(prefix) - len(suffix)
			if available <= 0 {
				available = discordMessageLimit
			}

			spaceLeft := available - current.Len()
			if spaceLeft <= 0 {
				if openFence != "" && !strings.HasSuffix(current.String(), "\n```") {
					current.WriteString("\n```")
				}
				flush()
				continue
			}

			if current.Len() == 0 && prefix != "" {
				current.WriteString(prefix)
				spaceLeft = available - current.Len()
			}

			if len(line) <= spaceLeft {
				current.WriteString(line)
				line = ""
				continue
			}

			splitAt := strings.LastIndex(line[:spaceLeft], "\n")
			if splitAt <= 0 {
				splitAt = strings.LastIndex(line[:spaceLeft], " ")
			}
			if splitAt <= 0 {
				splitAt = spaceLeft
			}

			current.WriteString(line[:splitAt])
			line = line[splitAt:]

			if openFence != "" && !strings.HasSuffix(current.String(), "\n```") {
				current.WriteString("\n```")
			}
			flush()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		appendWithFence(line)

		if strings.HasPrefix(trimmed, "```") {
			if openFence == "" {
				openFence = trimmed
				if openFence == "```" {
					openFence = "```"
				}
			} else {
				openFence = ""
			}
		}
	}

	if openFence != "" && !strings.HasSuffix(current.String(), "\n```") {
		current.WriteString("\n```")
	}
	flush()

	if len(chunks) == 0 {
		return []string{text}
	}

	return chunks
}
