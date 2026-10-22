# tagrename

Renames audio files in a folder using the metadata embedded in their tags, with a filename template you control.

## Requirements

- `ffprobe`, which ships with ffmpeg. Tags are read via `ffprobe -show_format`.
  - Debian/Ubuntu: `sudo apt install ffmpeg`
  - macOS: `brew install ffmpeg`
- Nothing else. tagrename is a single static binary with no third-party dependencies.

## Installation

### One-line install (Linux and macOS, amd64 and arm64)

```sh
curl -fsSL https://raw.githubusercontent.com/kitsu-agent/tagrename/main/install.sh | bash
```

This downloads the latest release archive for your OS and architecture, installs the
binary to `~/.local/bin/tagrename`, and warns you if `ffprobe` is missing. Override the
target directory with `INSTALL_DIR=/usr/local/bin` if you want it elsewhere.

### With Go

```sh
go install github.com/kitsu-agent/tagrename@latest
```

### From source

```sh
git clone https://github.com/kitsu-agent/tagrename.git
cd tagrename
make build && make install   # installs to ~/.local/bin, no sudo needed
```

`make install` writes to `$(HOME)/.local/bin`. To put the binary somewhere else, copy it
yourself: `install -m 0755 tagrename /usr/local/bin/`.

## Usage

```
tagrename [-f|--format TEMPLATE] [-e|--ext EXT] [-m|--mp3] [-r|--recursive] [-c|--config FILE] [DIR]
```

`DIR` defaults to the current directory. Every file in that directory whose extension
matches the selected audio extension is probed and renamed. With `-r/--recursive`, every
matching file in the tree below `DIR` is processed instead, each renamed inside its own
folder.

### Examples

```sh
# Rename every FLAC file in an album folder (default template)
tagrename ~/Music/Boards\ of\ Canada/Music\ Has\ the\ Right\ to\ Children

# MP3 instead of FLAC
tagrename -m ~/Music/Album
tagrename --ext mp3 ~/Music/Album

# Custom template: "03 - Roygbiv.flac"
tagrename -f '<track_number> - <song>' ~/Music/Album

# Per-disc folders, disc number in the name
tagrename -f '<disc>-<track_number> <song>' ~/Music/Album

# Whole library: every album below ~/Music, each renamed in place
tagrename -r ~/Music
```

## Placeholders

| Placeholder | Meaning | Notes |
|---|---|---|
| `<artist>` | Track artist | Falls back through the tag chain below |
| `<album>` | Album title | Falls back to the folder name |
| `<track_number>` | Track number, padded | Alias: `<track>` |
| `<song>` | Track title | Alias: `<title>` |
| `<disc>` | Disc number, unpadded | Alias: `<disc_number>` |

Unknown placeholders in a template are rejected before anything is renamed:

```
$ tagrename -f '<artist> - <bogus>' .
error: unknown placeholder <bogus>
```

A stray `<` with no matching `>` is treated as literal text, so `<3` in a template is fine.

## Multi-disc numbering

The default `<track_number>` value depends on the album:

- **Single-disc album** — two digits: track 1 → `01`, track 12 → `12`.
- **Multi-disc album** — three digits, disc and track combined: disc 1 track 1 → `101`,
  disc 2 track 4 → `204`.

An album counts as multi-disc when any of its files reports `DISCTOTAL`/`TOTALDISCS`
greater than 1, or a disc number greater than 1. The decision is made per album, so the
files of one album are always numbered consistently, regardless of the order they are
processed in.

## Config file

Defaults can be set once in a config file:

```
~/.config/tagrename/config
```

or, when `XDG_CONFIG_HOME` is set:

```
$XDG_CONFIG_HOME/tagrename/config
```

Format is one `key = value` per line. `#` and `;` start comments. Quotes around values
are optional and stripped.

```ini
format = "<artist> - <album> - <track_number> <song>"
ext = "flac"
```

Pass a different file with `-c/--config`. A missing config file is not an error unless you
passed it explicitly with `-c`.

**Precedence:** command-line flags > config file > built-in defaults. Note that `-m/--mp3`
beats the config's `ext`, and an explicit `-e/--ext` beats `-m`.

## Tag fallbacks

- **Artist:** `ARTIST` → `ARTISTS` → `ALBUM_ARTIST` → `ALBUMARTIST`
- **Album:** `ALBUM`; if that tag is empty, the containing folder's name is used
- **Title:** `TITLE`
- **Track:** `TRACK` → `TRACKNUMBER` (values like `3/12` are read as `3`)
- **Disc:** `DISC` → `DISCNUMBER`; missing means disc 1
- **Disc total:** `DISCTOTAL` → `TOTALDISCS`; missing means 1

Tag names are matched case-insensitively.

## Behavior

- Files missing artist, album, title or track number are **skipped**, with a message on
  stderr. Nothing is renamed if any of those four is unknown.
- A file whose target name already exists is **skipped**, never overwritten.
- A file already carrying the target name is reported as `ok:` and left alone.
- Because of the two rules above, running tagrename twice is safe: the second run renames
  nothing. It is idempotent.
- `/` in a rendered name is replaced with `-`, since it cannot appear in a filename.
- Files are renamed inside their own directory. Without `-r`, only files directly in
  `DIR` are touched; with `-r`, the whole tree below `DIR` is walked and every album is
  renamed where it lives. Multi-disc detection still groups by artist + album, so the
  numbers stay consistent across the discs of one release wherever they sit.
- `-r` follows real directories only; symlinked directories are not descended into.
- An unreadable subdirectory is reported on stderr and skipped; the rest of the run
  continues. A missing or unreadable `DIR` itself is a fatal error.

## License

MIT. See [LICENSE](LICENSE).
