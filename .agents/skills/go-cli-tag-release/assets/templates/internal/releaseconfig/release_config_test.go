package releaseconfig

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type goreleaserConfig struct {
	Builds []struct {
		ID      string   `yaml:"id"`
		Ldflags []string `yaml:"ldflags"`
	} `yaml:"builds"`
	Archives []struct {
		ID           string `yaml:"id"`
		NameTemplate string `yaml:"name_template"`
	} `yaml:"archives"`
}

func TestGoReleaserConfigEmbedsReleaseVersion(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)

	for _, build := range config.Builds {
		if build.ID != "__BINARY_NAME__" {
			continue
		}

		found := false
		for _, ldflag := range build.Ldflags {
			if ldflag == "-s -w -X __MODULE_PATH__/version.Version={{ .Version }}" {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("build %q is missing the release version ldflags", build.ID)
		}

		return
	}

	t.Fatal("target build not found in .goreleaser.yaml")
}

func TestGoReleaserConfigUsesStableArchiveNameTemplate(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)
	if len(config.Archives) == 0 {
		t.Fatal("archives section not found in .goreleaser.yaml")
	}

	const want = "{{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_{{ if eq .Arch \"amd64\" }}x86_64{{ else }}{{ .Arch }}{{ end }}"
	if config.Archives[0].NameTemplate != want {
		t.Fatalf("unexpected archive name template: %s", config.Archives[0].NameTemplate)
	}

	if config.Archives[0].ID != "release-archives" {
		t.Fatalf("unexpected archive id: %s", config.Archives[0].ID)
	}
}

func loadGoReleaserConfig(t *testing.T) goreleaserConfig {
	t.Helper()

	configPath := filepath.Join("..", "..", ".goreleaser.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read %s: %v", configPath, err)
	}

	var config goreleaserConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("unmarshal %s: %v", configPath, err)
	}

	return config
}
