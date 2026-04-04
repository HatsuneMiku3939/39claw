package codex

import (
	"fmt"
	"strings"
)

type InputPartType string

const (
	InputPartTypeText       InputPartType = "text"
	InputPartTypeLocalImage InputPartType = "local_image"
)

type InputPart struct {
	Type InputPartType
	Text string
	Path string
}

type Input struct {
	Parts []InputPart
}

type normalizedInput struct {
	Prompt string
	Images []string
}

func TextInput(text string) Input {
	return MultiPartInput(TextPart(text))
}

func MultiPartInput(parts ...InputPart) Input {
	cloned := append([]InputPart(nil), parts...)
	return Input{Parts: cloned}
}

func TextPart(text string) InputPart {
	return InputPart{
		Type: InputPartTypeText,
		Text: text,
	}
}

func LocalImagePart(path string) InputPart {
	return InputPart{
		Type: InputPartTypeLocalImage,
		Path: path,
	}
}

func (i Input) normalize() (normalizedInput, error) {
	if len(i.Parts) == 0 {
		return normalizedInput{}, nil
	}

	textParts := make([]string, 0, len(i.Parts))
	images := make([]string, 0, len(i.Parts))

	for _, part := range i.Parts {
		switch part.Type {
		case InputPartTypeText:
			textParts = append(textParts, part.Text)
		case InputPartTypeLocalImage:
			if part.Path == "" {
				return normalizedInput{}, fmt.Errorf("local image input requires a path")
			}
			images = append(images, part.Path)
		default:
			return normalizedInput{}, fmt.Errorf("unsupported input part type %q", part.Type)
		}
	}

	return normalizedInput{
		Prompt: strings.Join(textParts, "\n\n"),
		Images: images,
	}, nil
}
