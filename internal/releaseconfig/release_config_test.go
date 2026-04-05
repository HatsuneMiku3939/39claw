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
		Main    string   `yaml:"main"`
		Binary  string   `yaml:"binary"`
		Ldflags []string `yaml:"ldflags"`
	} `yaml:"builds"`
	Archives []struct {
		ID           string   `yaml:"id"`
		IDs          []string `yaml:"ids"`
		NameTemplate string   `yaml:"name_template"`
		Files        []string `yaml:"files"`
	} `yaml:"archives"`
	NFPMS []struct {
		ID               string   `yaml:"id"`
		PackageName      string   `yaml:"package_name"`
		IDs              []string `yaml:"ids"`
		Formats          []string `yaml:"formats"`
		FileNameTemplate string   `yaml:"file_name_template"`
	} `yaml:"nfpms"`
	HomebrewCasks []struct {
		Name string   `yaml:"name"`
		IDs  []string `yaml:"ids"`
	} `yaml:"homebrew_casks"`
	Release struct {
		Draft bool `yaml:"draft"`
	} `yaml:"release"`
}

func TestGoReleaserConfigTargets39clawBuild(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)

	if len(config.Builds) != 1 {
		t.Fatalf("build count = %d, want 1", len(config.Builds))
	}

	build := config.Builds[0]
	if build.ID != "39claw" {
		t.Fatalf("build id = %q, want %q", build.ID, "39claw")
	}

	if build.Main != "./cmd/39claw" {
		t.Fatalf("build main = %q, want %q", build.Main, "./cmd/39claw")
	}

	if build.Binary != "39claw" {
		t.Fatalf("build binary = %q, want %q", build.Binary, "39claw")
	}
}

func TestGoReleaserConfigEmbedsReleaseVersion(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)
	build := config.Builds[0]

	const want = "-s -w -X github.com/HatsuneMiku3939/39claw/version.Version={{ .Version }}"
	for _, ldflag := range build.Ldflags {
		if ldflag == want {
			return
		}
	}

	t.Fatalf("build %q is missing ldflags %q", build.ID, want)
}

func TestGoReleaserConfigUsesStableArchiveContract(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)
	if len(config.Archives) != 1 {
		t.Fatalf("archive count = %d, want 1", len(config.Archives))
	}

	archive := config.Archives[0]
	if archive.ID != "release-archives" {
		t.Fatalf("archive id = %q, want %q", archive.ID, "release-archives")
	}

	if len(archive.IDs) != 1 || archive.IDs[0] != "39claw" {
		t.Fatalf("archive ids = %v, want [39claw]", archive.IDs)
	}

	const wantTemplate = "{{ .ProjectName }}_{{ .Version }}_{{ title .Os }}_{{ if eq .Arch \"amd64\" }}x86_64{{ else }}{{ .Arch }}{{ end }}"
	if archive.NameTemplate != wantTemplate {
		t.Fatalf("archive name template = %q, want %q", archive.NameTemplate, wantTemplate)
	}

	if len(archive.Files) != 2 || archive.Files[0] != "README*" || archive.Files[1] != "LICENSE*" {
		t.Fatalf("archive files = %v, want [README* LICENSE*]", archive.Files)
	}
}

func TestGoReleaserConfigProducesLinuxPackages(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)

	for _, pkg := range config.NFPMS {
		if pkg.ID != "linux-packages" {
			continue
		}

		if pkg.PackageName != "39claw" {
			t.Fatalf("package name = %q, want %q", pkg.PackageName, "39claw")
		}

		assertContainsAll(t, pkg.IDs, []string{"39claw"})
		assertContainsAll(t, pkg.Formats, []string{"deb", "rpm"})

		if pkg.FileNameTemplate != "{{ .ConventionalFileName }}" {
			t.Fatalf("file name template = %q, want %q", pkg.FileNameTemplate, "{{ .ConventionalFileName }}")
		}

		return
	}

	t.Fatal("linux-packages nfpms entry not found in .goreleaser.yaml")
}

func TestGoReleaserConfigHomebrewCaskUsesArchiveIDs(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)

	archiveIDs := make(map[string]struct{}, len(config.Archives))
	for _, archive := range config.Archives {
		if archive.ID == "" {
			continue
		}

		archiveIDs[archive.ID] = struct{}{}
	}

	if len(archiveIDs) == 0 {
		t.Fatal("expected at least one archive id in .goreleaser.yaml")
	}

	for _, cask := range config.HomebrewCasks {
		if cask.Name != "39claw" {
			continue
		}

		if len(cask.IDs) == 0 {
			t.Fatal("homebrew cask must reference at least one archive id")
		}

		for _, id := range cask.IDs {
			if _, ok := archiveIDs[id]; !ok {
				t.Fatalf("homebrew cask references unknown archive id %q", id)
			}
		}

		return
	}

	t.Fatal("39claw homebrew cask not found in .goreleaser.yaml")
}

func TestGoReleaserConfigKeepsDraftRelease(t *testing.T) {
	t.Parallel()

	config := loadGoReleaserConfig(t)

	if !config.Release.Draft {
		t.Fatal("release.draft = false, want true")
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

func assertContainsAll(t *testing.T, got []string, want []string) {
	t.Helper()

	gotSet := make(map[string]struct{}, len(got))
	for _, value := range got {
		gotSet[value] = struct{}{}
	}

	for _, value := range want {
		if _, ok := gotSet[value]; !ok {
			t.Fatalf("expected %q in %v", value, got)
		}
	}
}
