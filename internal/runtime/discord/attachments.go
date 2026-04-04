package discord

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type attachmentHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var supportedImageExtensions = map[string]struct{}{
	".apng": {},
	".bmp":  {},
	".gif":  {},
	".jpeg": {},
	".jpg":  {},
	".png":  {},
	".tif":  {},
	".tiff": {},
	".webp": {},
}

func prepareImageAttachments(
	ctx context.Context,
	client attachmentHTTPClient,
	attachments []*discordgo.MessageAttachment,
) ([]string, func(), error) {
	imageAttachments := filterImageAttachments(attachments)
	if len(imageAttachments) == 0 {
		return nil, nil, nil
	}

	tempDir, err := os.MkdirTemp("", "39claw-discord-images-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temporary image directory: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	paths := make([]string, 0, len(imageAttachments))
	for index, attachment := range imageAttachments {
		path, err := downloadAttachment(ctx, client, tempDir, index, attachment)
		if err != nil {
			cleanup()
			return nil, nil, err
		}

		paths = append(paths, path)
	}

	return paths, cleanup, nil
}

func filterImageAttachments(attachments []*discordgo.MessageAttachment) []*discordgo.MessageAttachment {
	filtered := make([]*discordgo.MessageAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		if isSupportedImageAttachment(attachment) {
			filtered = append(filtered, attachment)
		}
	}

	return filtered
}

func isSupportedImageAttachment(attachment *discordgo.MessageAttachment) bool {
	if attachment == nil {
		return false
	}

	contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	_, ok := supportedImageExtensions[strings.ToLower(filepath.Ext(attachment.Filename))]
	return ok
}

func downloadAttachment(
	ctx context.Context,
	client attachmentHTTPClient,
	tempDir string,
	index int,
	attachment *discordgo.MessageAttachment,
) (string, error) {
	if client == nil {
		return "", fmt.Errorf("attachment http client must not be nil")
	}

	url := strings.TrimSpace(attachment.URL)
	if url == "" {
		return "", fmt.Errorf("attachment %q is missing a download URL", attachment.Filename)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build attachment request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download attachment %q: %w", attachment.Filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download attachment %q: unexpected status %s", attachment.Filename, resp.Status)
	}

	filePath := filepath.Join(tempDir, fmt.Sprintf("attachment-%02d%s", index+1, attachmentExtension(attachment)))
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create attachment file %q: %w", filePath, err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(filePath)
		return "", fmt.Errorf("write attachment file %q: %w", filePath, err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("close attachment file %q: %w", filePath, err)
	}

	return filePath, nil
}

func attachmentExtension(attachment *discordgo.MessageAttachment) string {
	if ext := strings.ToLower(filepath.Ext(attachment.Filename)); ext != "" {
		return ext
	}

	if attachment.ContentType != "" {
		extensions, err := mime.ExtensionsByType(attachment.ContentType)
		if err == nil && len(extensions) > 0 {
			return extensions[0]
		}
	}

	return ".img"
}
