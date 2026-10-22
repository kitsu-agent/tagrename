package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNum(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"plain", "3", 3},
		{"zero padded", "03", 3},
		{"track slash total", "3/12", 3},
		{"empty", "", 0},
		{"not a number", "abc", 0},
		{"surrounding spaces", " 7 ", 7},
		{"two digits", "10", 10},
		{"disc slash total", "2/2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseNum(tt.in); got != tt.want {
				t.Fatalf("parseNum(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeExt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"uppercase", "FLAC", "flac"},
		{"leading dot", ".mp3", "mp3"},
		{"mixed case and spaces", " M4A ", "m4a"},
		{"already normalized", "ogg", "ogg"},
		{"dot only", ".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeExt(tt.in); got != tt.want {
				t.Fatalf("normalizeExt(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		wantErr bool
	}{
		{"default template", defaultFormat, false},
		{"artist and song", "<artist> - <song>", false},
		{"unknown placeholder", "<foo>", true},
		{"stray opening bracket is literal", "Track < 5", false},
		{"empty template", "", false},
		{"plain text", "just a name", false},
		{"all placeholders", "<artist> <album> <track_number> <song> <disc>", false},
		{"aliases", "<track> - <title> - <disc_number>", false},
		{"unknown among known", "<artist> - <bogus>", true},
		{"whitespace inside token", "< artist >", false},
		{"uppercase token", "<ARTIST>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFormat(tt.tmpl)
			if tt.wantErr && err == nil {
				t.Fatalf("validateFormat(%q) = nil, want error", tt.tmpl)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateFormat(%q) = %v, want nil", tt.tmpl, err)
			}
		})
	}
}

func TestValidateFormatUnknownPlaceholderMessage(t *testing.T) {
	err := validateFormat("<foo>")
	if err == nil {
		t.Fatal("expected error for unknown placeholder")
	}
	if !strings.Contains(err.Error(), "<foo>") {
		t.Fatalf("error %q does not name the placeholder", err.Error())
	}
}

func TestRenderFormat(t *testing.T) {
	s := song{
		path:      "/tmp/x.flac",
		artist:    "Boards of Canada",
		album:     "Music Has the Right to Children",
		title:     "Roygbiv",
		track:     3,
		disc:      1,
		discTotal: 1,
	}

	tests := []struct {
		name string
		tmpl string
		nn   string
		want string
	}{
		{
			name: "default template",
			tmpl: defaultFormat,
			nn:   "03",
			want: "Boards of Canada - Music Has the Right to Children - 03 Roygbiv",
		},
		{
			name: "track alias",
			tmpl: "<track> - <song>",
			nn:   "03",
			want: "03 - Roygbiv",
		},
		{
			name: "title alias",
			tmpl: "<artist> - <title>",
			nn:   "03",
			want: "Boards of Canada - Roygbiv",
		},
		{
			name: "disc placeholder",
			tmpl: "d<disc>/<track_number> <song>",
			nn:   "103",
			want: "d1/103 Roygbiv",
		},
		{
			name: "disc_number alias",
			tmpl: "<disc_number>-<track_number>",
			nn:   "103",
			want: "1-103",
		},
		{
			name: "multiple tokens",
			tmpl: "<artist> - <album> - <track_number> <song>",
			nn:   "103",
			want: "Boards of Canada - Music Has the Right to Children - 103 Roygbiv",
		},
		{
			name: "no placeholders",
			tmpl: "fixed name",
			nn:   "03",
			want: "fixed name",
		},
		{
			name: "stray bracket kept literally",
			tmpl: "<artist> <3",
			nn:   "03",
			want: "Boards of Canada <3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderFormat(tt.tmpl, s, tt.nn); got != tt.want {
				t.Fatalf("renderFormat(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestRenderFormatSingleDigitPadding(t *testing.T) {
	s := song{
		path:      "/tmp/x.flac",
		artist:    "A",
		album:     "B",
		title:     "T",
		track:     1,
		disc:      1,
		discTotal: 2,
	}

	// Caller supplies the padded number; renderFormat must not add its own padding.
	if got := renderFormat(defaultFormat, s, "101"); got != "A - B - 101 T" {
		t.Fatalf("renderFormat with triple digits = %q", got)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("parses keys ignores comments and blanks", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config")
		body := strings.Join([]string{
			"# a comment",
			"; another comment",
			"",
			`format = "<track_number> - <song>"`,
			`  ext = "FLAC"  `,
			"novalue line ignored",
			"",
		}, "\n")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if got, want := cfg["format"], "<track_number> - <song>"; got != want {
			t.Fatalf("format = %q, want %q", got, want)
		}
		if got, want := cfg["ext"], "FLAC"; got != want {
			t.Fatalf("ext = %q, want %q (quotes should be stripped)", got, want)
		}
		if len(cfg) != 2 {
			t.Fatalf("expected 2 keys, got %d: %v", len(cfg), cfg)
		}
	})

	t.Run("unquoted values and uppercase keys", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config")
		if err := os.WriteFile(path, []byte("FORMAT=<song>\nExt=mp3\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if got := cfg["format"]; got != "<song>" {
			t.Fatalf("format = %q", got)
		}
		if got := cfg["ext"]; got != "mp3" {
			t.Fatalf("ext = %q", got)
		}
	})

	t.Run("missing default config is not an error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		cfg, err := loadConfig("")
		if err != nil {
			t.Fatalf("loadConfig(\"\") returned error: %v", err)
		}
		if len(cfg) != 0 {
			t.Fatalf("expected empty config, got %v", cfg)
		}
	})

	t.Run("explicit missing config is an error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		path := filepath.Join(t.TempDir(), "does-not-exist")

		cfg, err := loadConfig(path)
		if err == nil {
			t.Fatalf("expected error for explicit missing path, got cfg %v", cfg)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("expected not-exist error, got %v", err)
		}
	})

	t.Run("reads default config from XDG_CONFIG_HOME", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		if err := os.MkdirAll(filepath.Join(xdg, "tagrename"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(xdg, "tagrename", "config"), []byte("ext = ogg\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		cfg, err := loadConfig("")
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		if got := cfg["ext"]; got != "ogg" {
			t.Fatalf("ext = %q, want ogg", got)
		}
	})
}

func TestDefaultConfigPath(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	want := filepath.Join(xdg, "tagrename", "config")
	if got := defaultConfigPath(); got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}

func TestFirstTag(t *testing.T) {
	tags := map[string]string{
		"artists": "Fallback Artist",
		"album":   "An Album",
	}

	if got := firstTag(tags, "artist", "artists", "albumartist"); got != "Fallback Artist" {
		t.Fatalf("firstTag fallback = %q", got)
	}
	if got := firstTag(tags, "album"); got != "An Album" {
		t.Fatalf("firstTag album = %q", got)
	}
	if got := firstTag(tags, "missing"); got != "" {
		t.Fatalf("firstTag missing = %q, want empty", got)
	}
}

func TestCollectAudioFiles(t *testing.T) {
	root := t.TempDir()

	mk := func(rel string) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		return path
	}

	a := mk("a.flac")
	mk("b.mp3")
	mk("notes.txt")
	mk("a.flac.bak")
	mk("E.FLAC")
	c := mk("sub/c.flac")
	d := mk("sub/deep/d.flac")

	t.Run("non-recursive sees only the top level", func(t *testing.T) {
		got, err := collectAudioFiles(root, "flac", false)
		if err != nil {
			t.Fatalf("collectAudioFiles: %v", err)
		}
		want := []string{filepath.Join(root, "E.FLAC"), a}
		if !equalPaths(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("recursive walks the whole tree", func(t *testing.T) {
		got, err := collectAudioFiles(root, "flac", true)
		if err != nil {
			t.Fatalf("collectAudioFiles: %v", err)
		}
		want := []string{filepath.Join(root, "E.FLAC"), a, c, d}
		if !equalPaths(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("recursive filters by extension", func(t *testing.T) {
		got, err := collectAudioFiles(root, "mp3", true)
		if err != nil {
			t.Fatalf("collectAudioFiles: %v", err)
		}
		want := []string{filepath.Join(root, "b.mp3")}
		if !equalPaths(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("empty directory yields nothing", func(t *testing.T) {
		empty := t.TempDir()
		for _, rec := range []bool{false, true} {
			got, err := collectAudioFiles(empty, "flac", rec)
			if err != nil {
				t.Fatalf("collectAudioFiles(recursive=%v): %v", rec, err)
			}
			if len(got) != 0 {
				t.Fatalf("recursive=%v: expected no files, got %v", rec, got)
			}
		}
	})

	t.Run("missing directory is an error in both modes", func(t *testing.T) {
		missing := filepath.Join(root, "nope")
		for _, rec := range []bool{false, true} {
			if _, err := collectAudioFiles(missing, "flac", rec); err == nil {
				t.Fatalf("recursive=%v: expected error for missing dir", rec)
			}
		}
	})

	t.Run("symlinked directories are not followed", func(t *testing.T) {
		link := filepath.Join(root, "link")
		if err := os.Symlink(filepath.Join(root, "sub"), link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		got, err := collectAudioFiles(root, "flac", true)
		if err != nil {
			t.Fatalf("collectAudioFiles: %v", err)
		}
		for _, p := range got {
			if strings.HasPrefix(p, link+string(filepath.Separator)) {
				t.Fatalf("walk followed symlinked directory: %s", p)
			}
		}
	})
}

func equalPaths(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
