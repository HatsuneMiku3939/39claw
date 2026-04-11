package discord

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type attachmentHTTPClientFunc func(req *http.Request) (*http.Response, error)

func (f attachmentHTTPClientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPrepareImageAttachmentsDownloadsSupportedImagesAndCleansUp(t *testing.T) {
	t.Parallel()

	requested := []string{}
	client := attachmentHTTPClientFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())

		var body string
		switch req.URL.String() {
		case "https://example.com/photo":
			body = "photo-bytes"
		case "https://example.com/diagram":
			body = "diagram-bytes"
		default:
			t.Fatalf("unexpected download url %q", req.URL.String())
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})

	paths, cleanup, err := prepareImageAttachments(context.Background(), client, []*discordgo.MessageAttachment{
		nil,
		{Filename: "notes.txt", URL: "https://example.com/notes", ContentType: "text/plain"},
		{Filename: "photo.jpeg", URL: "https://example.com/photo", ContentType: "image/jpeg"},
		{Filename: "diagram.PNG", URL: "https://example.com/diagram"},
	})
	if err != nil {
		t.Fatalf("prepareImageAttachments() error = %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup = nil, want cleanup func")
	}

	if len(paths) != 2 {
		t.Fatalf("downloaded path count = %d, want %d", len(paths), 2)
	}
	if len(requested) != 2 {
		t.Fatalf("download request count = %d, want %d", len(requested), 2)
	}

	if filepath.Ext(paths[0]) != ".jpeg" {
		t.Fatalf("first path extension = %q, want %q", filepath.Ext(paths[0]), ".jpeg")
	}
	if filepath.Ext(paths[1]) != ".png" {
		t.Fatalf("second path extension = %q, want %q", filepath.Ext(paths[1]), ".png")
	}

	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", paths[0], err)
	}
	if string(data) != "photo-bytes" {
		t.Fatalf("first attachment contents = %q, want %q", string(data), "photo-bytes")
	}

	tempDir := filepath.Dir(paths[0])
	cleanup()

	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temp dir still exists after cleanup: err = %v", err)
	}
}

func TestDownloadAttachmentWritesFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	client := attachmentHTTPClientFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("request method = %q, want %q", req.Method, http.MethodGet)
		}
		if req.URL.String() != "https://example.com/image" {
			t.Fatalf("request url = %q, want %q", req.URL.String(), "https://example.com/image")
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("image-bytes")),
		}, nil
	})

	path, err := downloadAttachment(context.Background(), client, tempDir, 0, &discordgo.MessageAttachment{
		Filename: "image.png",
		URL:      "https://example.com/image",
	})
	if err != nil {
		t.Fatalf("downloadAttachment() error = %v", err)
	}

	if filepath.Ext(path) != ".png" {
		t.Fatalf("downloaded file extension = %q, want %q", filepath.Ext(path), ".png")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("downloaded contents = %q, want %q", string(data), "image-bytes")
	}
}

func TestDownloadAttachmentRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	_, err := downloadAttachment(context.Background(), nil, tempDir, 0, &discordgo.MessageAttachment{
		Filename: "image.png",
		URL:      "https://example.com/image",
	})
	if err == nil || !strings.Contains(err.Error(), "attachment http client must not be nil") {
		t.Fatalf("nil client error = %v, want nil client validation", err)
	}

	_, err = downloadAttachment(context.Background(), attachmentHTTPClientFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatal("Do() should not be called when URL is missing")
		return nil, nil
	}), tempDir, 0, &discordgo.MessageAttachment{Filename: "image.png"})
	if err == nil || !strings.Contains(err.Error(), "missing a download URL") {
		t.Fatalf("missing URL error = %v, want missing URL validation", err)
	}
}

func TestDownloadAttachmentRejectsUnexpectedStatus(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	client := attachmentHTTPClientFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Body:       io.NopCloser(strings.NewReader("bad gateway")),
		}, nil
	})

	_, err := downloadAttachment(context.Background(), client, tempDir, 0, &discordgo.MessageAttachment{
		Filename: "image.png",
		URL:      "https://example.com/image",
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected status 502 Bad Gateway") {
		t.Fatalf("downloadAttachment() error = %v, want unexpected status error", err)
	}
}

func TestAttachmentExtension(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		attachment *discordgo.MessageAttachment
		want       string
	}{
		{
			name:       "filename extension wins",
			attachment: &discordgo.MessageAttachment{Filename: "PHOTO.JPG", ContentType: "image/png"},
			want:       ".jpg",
		},
		{
			name:       "content type fallback",
			attachment: &discordgo.MessageAttachment{Filename: "photo", ContentType: "image/png"},
			want:       ".png",
		},
		{
			name:       "default extension",
			attachment: &discordgo.MessageAttachment{Filename: "photo", ContentType: "application/octet-stream"},
			want:       ".img",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := attachmentExtension(tc.attachment); got != tc.want {
				t.Fatalf("attachmentExtension() = %q, want %q", got, tc.want)
			}
		})
	}
}
