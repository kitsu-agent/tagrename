// tagrename renames audio files using a configurable filename template.
//
// Default template: <artist> - <album> - <track_number> <song>
// Multi-disc albums use triple digits (disc 1 track 1 -> 101, disc 2 track 4 -> 204);
// single-disc albums use two digits (track 1 -> 01).
//
// The template can be changed with -f/--format or the "format" key in the config.
// The audio extension defaults to flac and can be changed with -e/--ext, -m/--mp3,
// or the "ext" config key.
//
// Usage: tagrename [-f|--format TEMPLATE] [-e|--ext EXT] [-m|--mp3] [-c|--config FILE] [DIR]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultFormat = "<artist> - <album> - <track_number> <song>"
	defaultExt    = "flac"
)

type ffprobeOutput struct {
	Format struct {
		Tags map[string]string `json:"tags"`
	} `json:"format"`
}

type song struct {
	path      string
	artist    string
	album     string
	title     string
	track     int
	disc      int
	discTotal int
}

type albumInfo struct {
	maxDisc   int
	discTotal int
}

var placeholders = map[string]bool{
	"artist":       true,
	"album":        true,
	"track_number": true,
	"track":        true,
	"song":         true,
	"title":        true,
	"disc":         true,
	"disc_number":  true,
}

func main() {
	var (
		formatFlag string
		extFlag    string
		mp3Flag    bool
		configFlag string
		recursive  bool
	)

	flag.StringVar(&formatFlag, "f", "", "filename template (default: "+defaultFormat+")")
	flag.StringVar(&formatFlag, "format", "", "filename template (default: "+defaultFormat+")")
	flag.StringVar(&extFlag, "e", "", "audio extension to process (default: "+defaultExt+")")
	flag.StringVar(&extFlag, "ext", "", "audio extension to process (default: "+defaultExt+")")
	flag.BoolVar(&mp3Flag, "m", false, "shorthand for --ext mp3")
	flag.BoolVar(&mp3Flag, "mp3", false, "shorthand for --ext mp3")
	flag.StringVar(&configFlag, "c", "", "config file path")
	flag.StringVar(&configFlag, "config", "", "config file path")
	flag.BoolVar(&recursive, "r", false, "recurse into subdirectories")
	flag.BoolVar(&recursive, "recursive", false, "recurse into subdirectories")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-f|--format TEMPLATE] [-e|--ext EXT] [-m|--mp3] [-r|--recursive] [-c|--config FILE] [DIR]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Renames audio files using a filename template.\n\n")
		fmt.Fprintf(os.Stderr, "Placeholders: <artist>, <album>, <track_number> (alias <track>), <song> (alias <title>), <disc>\n")
		fmt.Fprintf(os.Stderr, "Config file (optional): %s\n\n", defaultConfigPath())
		fmt.Fprintf(os.Stderr, "  format = \"%s\"\n", defaultFormat)
		fmt.Fprintf(os.Stderr, "  ext = \"%s\"\n\n", defaultExt)
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := loadConfig(configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ext := defaultExt
	switch {
	case extFlag != "":
		ext = normalizeExt(extFlag)
	case mp3Flag:
		ext = "mp3"
	case cfg["ext"] != "":
		ext = normalizeExt(cfg["ext"])
	}
	if ext == "" {
		fmt.Fprintln(os.Stderr, "error: empty extension")
		os.Exit(1)
	}

	format := defaultFormat
	if formatFlag != "" {
		format = formatFlag
	} else if cfg["format"] != "" {
		format = cfg["format"]
	}
	if err := validateFormat(format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	paths, err := collectAudioFiles(dir, ext, recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var songs []song
	albums := map[string]*albumInfo{}

	for _, path := range paths {
		tags, err := probeTags(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip (probe failed): %s: %v\n", path, err)
			continue
		}

		artist := firstTag(tags, "artist", "artists", "album_artist", "albumartist")
		album := firstTag(tags, "album")
		if album == "" {
			album = filepath.Base(filepath.Dir(path))
		}
		title := firstTag(tags, "title")
		track := parseNum(firstTag(tags, "track", "tracknumber"))
		disc := parseNum(firstTag(tags, "disc", "discnumber"))
		discTotal := parseNum(firstTag(tags, "disctotal", "totaldiscs"))

		if disc == 0 {
			disc = 1
		}
		if discTotal == 0 {
			discTotal = 1
		}

		if artist == "" || album == "" || title == "" || track == 0 {
			fmt.Fprintf(os.Stderr, "skip (missing tags): %s\n", path)
			continue
		}

		songs = append(songs, song{
			path:      path,
			artist:    artist,
			album:     album,
			title:     title,
			track:     track,
			disc:      disc,
			discTotal: discTotal,
		})

		key := artist + "\x00" + album
		info := albums[key]
		if info == nil {
			info = &albumInfo{}
			albums[key] = info
		}
		if discTotal > info.discTotal {
			info.discTotal = discTotal
		}
		if disc > info.maxDisc {
			info.maxDisc = disc
		}
	}

	for _, s := range songs {
		info := albums[s.artist+"\x00"+s.album]

		var nn string
		if info.discTotal > 1 || info.maxDisc > 1 {
			nn = fmt.Sprintf("%03d", s.disc*100+s.track)
		} else {
			nn = fmt.Sprintf("%02d", s.track)
		}

		name := renderFormat(format, s, nn)
		name = strings.ReplaceAll(name, "/", "-")
		// Always rename next to the file itself, so a recursive run keeps each
		// album in its own directory.
		newPath := filepath.Join(filepath.Dir(s.path), name+"."+ext)

		switch {
		case newPath == s.path:
			fmt.Printf("ok: %s\n", filepath.Base(s.path))
		case exists(newPath):
			fmt.Fprintf(os.Stderr, "skip (exists): %s\n", newPath)
		default:
			if err := os.Rename(s.path, newPath); err != nil {
				fmt.Fprintf(os.Stderr, "error renaming %s: %v\n", s.path, err)
				continue
			}
			fmt.Printf("renamed: %s -> %s\n", filepath.Base(s.path), filepath.Base(newPath))
		}
	}
}

// collectAudioFiles returns the paths of audio files directly inside dir, or of
// every audio file below dir when recursive is set. Directory entries are walked
// lexically, so results are deterministic; symlinked directories are not followed.
func collectAudioFiles(dir, ext string, recursive bool) ([]string, error) {
	if !recursive {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		var paths []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), "."+ext) {
				continue
			}
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
		return paths, nil
	}

	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			fmt.Fprintf(os.Stderr, "skip (walk failed): %s: %v\n", path, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), "."+ext) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// renderFormat substitutes <placeholder> tokens in the template.
func renderFormat(tmpl string, s song, nn string) string {
	values := map[string]string{
		"artist":       s.artist,
		"album":        s.album,
		"track_number": nn,
		"track":        nn,
		"song":         s.title,
		"title":        s.title,
		"disc":         strconv.Itoa(s.disc),
		"disc_number":  strconv.Itoa(s.disc),
	}

	var b strings.Builder
	for i := 0; i < len(tmpl); {
		if tmpl[i] == '<' {
			if j := strings.IndexByte(tmpl[i:], '>'); j >= 0 {
				token := strings.ToLower(strings.TrimSpace(tmpl[i+1 : i+j]))
				b.WriteString(values[token])
				i += j + 1
				continue
			}
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String()
}

// validateFormat rejects templates containing unknown <placeholders>.
func validateFormat(tmpl string) error {
	for i := 0; i < len(tmpl); {
		if tmpl[i] == '<' {
			if j := strings.IndexByte(tmpl[i:], '>'); j >= 0 {
				token := strings.ToLower(strings.TrimSpace(tmpl[i+1 : i+j]))
				if !placeholders[token] {
					return fmt.Errorf("unknown placeholder <%s>", token)
				}
				i += j + 1
				continue
			}
		}
		i++
	}
	return nil
}

func probeTags(path string) (map[string]string, error) {
	out, err := exec.Command("ffprobe", "-v", "quiet",
		"-print_format", "json", "-show_format", path).Output()
	if err != nil {
		return nil, err
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	tags := make(map[string]string, len(parsed.Format.Tags))
	for k, v := range parsed.Format.Tags {
		tags[strings.ToLower(k)] = v
	}
	return tags, nil
}

func firstTag(tags map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := tags[k]; v != "" {
			return v
		}
	}
	return ""
}

// parseNum extracts the leading number from values like "3" or "3/12".
func parseNum(s string) int {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
	n, _ := strconv.Atoi(s)
	return n
}

func normalizeExt(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(s, ".")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func defaultConfigPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tagrename", "config")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "tagrename", "config")
}

// loadConfig reads simple "key = value" lines. A missing config file is not an
// error unless explicitly requested with -c/--config.
func loadConfig(path string) (map[string]string, error) {
	explicit := path != ""
	if !explicit {
		path = defaultConfigPath()
	}

	cfg := map[string]string{}
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return cfg, nil
		}
		return nil, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		cfg[key] = value
	}
	return cfg, nil
}
