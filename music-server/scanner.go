package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Library struct {
	Genres []*Genre
	byPath map[string]*Album
}

type Genre struct {
	Name    string
	Artists []*Artist
}

type Artist struct {
	Name   string
	Genre  *Genre
	Albums []*Album
}

type Album struct {
	Title    string
	Path     string // absolute path on disk
	Artist   *Artist
	CoverArt string // relative path within Path, "" if none
	Tracks   []*Track
}

type Track struct {
	Number   int
	Title    string
	Filename string // relative path from album dir (may include subdir/)
	Format   string // "flac", "mp3", etc.
}

var trackNumRe = regexp.MustCompile(`^0*(\d+)`)

var audioExts = map[string]bool{
	".flac": true, ".mp3": true, ".ape": true, ".wav": true,
	".ogg": true, ".m4a": true, ".aac": true, ".wma": true,
}

var stdCoverNames = map[string]bool{
	"folder.jpg": true, "folder.jpeg": true, "folder.png": true,
	"cover.jpg": true, "cover.jpeg": true, "cover.png": true,
	"front.jpg": true, "front.jpeg": true, "front.png": true,
}

func scanLibrary(root string) (*Library, error) {
	lib := &Library{byPath: make(map[string]*Album)}

	genreDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, gd := range genreDirs {
		if !gd.IsDir() || strings.HasPrefix(gd.Name(), ".") {
			continue
		}
		g := &Genre{Name: gd.Name()}
		genrePath := filepath.Join(root, gd.Name())

		artistDirs, err := os.ReadDir(genrePath)
		if err != nil {
			continue
		}
		for _, ad := range artistDirs {
			if !ad.IsDir() {
				continue
			}
			ar := &Artist{Name: ad.Name(), Genre: g}
			artistPath := filepath.Join(genrePath, ad.Name())

			albumDirs, err := os.ReadDir(artistPath)
			if err != nil {
				continue
			}
			for _, ald := range albumDirs {
				if !ald.IsDir() {
					continue
				}
				albumPath := filepath.Join(artistPath, ald.Name())
				alb := scanAlbum(albumPath, ald.Name(), ar)
				if len(alb.Tracks) == 0 {
					continue
				}
				ar.Albums = append(ar.Albums, alb)
				lib.byPath[albumPath] = alb
			}
			if len(ar.Albums) > 0 {
				sort.Slice(ar.Albums, func(i, j int) bool {
					return ar.Albums[i].Title < ar.Albums[j].Title
				})
				g.Artists = append(g.Artists, ar)
			}
		}
		if len(g.Artists) > 0 {
			sort.Slice(g.Artists, func(i, j int) bool {
				return g.Artists[i].Name < g.Artists[j].Name
			})
			lib.Genres = append(lib.Genres, g)
		}
	}
	sort.Slice(lib.Genres, func(i, j int) bool {
		return lib.Genres[i].Name < lib.Genres[j].Name
	})
	return lib, nil
}

func scanAlbum(path, name string, ar *Artist) *Album {
	alb := &Album{Title: name, Path: path, Artist: ar}

	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(path, p)
		lower := strings.ToLower(d.Name())
		ext := strings.ToLower(filepath.Ext(d.Name()))

		if alb.CoverArt == "" && stdCoverNames[lower] {
			alb.CoverArt = rel
		}
		if audioExts[ext] {
			alb.Tracks = append(alb.Tracks, parseTrack(rel))
		}
		return nil
	})

	// Fallback: first image if no standard cover name matched
	if alb.CoverArt == "" {
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || alb.CoverArt != "" {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
				rel, _ := filepath.Rel(path, p)
				alb.CoverArt = rel
			}
			return nil
		})
	}

	sort.Slice(alb.Tracks, func(i, j int) bool {
		ti, tj := alb.Tracks[i], alb.Tracks[j]
		if ti.Number != tj.Number {
			return ti.Number < tj.Number
		}
		return ti.Filename < tj.Filename
	})
	return alb
}

func parseTrack(relPath string) *Track {
	t := &Track{Filename: relPath}
	base := filepath.Base(relPath)
	ext := filepath.Ext(base)
	t.Format = strings.TrimPrefix(strings.ToLower(ext), ".")
	stem := strings.TrimSuffix(base, ext)

	if m := trackNumRe.FindStringSubmatch(stem); m != nil {
		n, _ := strconv.Atoi(m[1])
		t.Number = n
		rest := strings.TrimLeft(stem[len(m[0]):], " .-_")
		t.Title = strings.TrimSpace(rest)
	} else {
		t.Title = stem
	}
	if t.Title == "" {
		t.Title = stem
	}
	return t
}

// Lookup helpers

func (lib *Library) findGenre(name string) *Genre {
	for _, g := range lib.Genres {
		if g.Name == name {
			return g
		}
	}
	return nil
}

func (lib *Library) findArtist(genreName, artistName string) *Artist {
	g := lib.findGenre(genreName)
	if g == nil {
		return nil
	}
	for _, ar := range g.Artists {
		if ar.Name == artistName {
			return ar
		}
	}
	return nil
}

func (lib *Library) findAlbum(genreName, artistName, albumTitle string) *Album {
	ar := lib.findArtist(genreName, artistName)
	if ar == nil {
		return nil
	}
	for _, alb := range ar.Albums {
		if alb.Title == albumTitle {
			return alb
		}
	}
	return nil
}

func (lib *Library) search(q string) (artists []*Artist, albums []*Album) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return
	}
	seenAr := map[string]bool{}
	seenAlb := map[string]bool{}
	for _, g := range lib.Genres {
		for _, ar := range g.Artists {
			nameMatch := strings.Contains(strings.ToLower(ar.Name), q)
			if nameMatch && !seenAr[ar.Name] {
				artists = append(artists, ar)
				seenAr[ar.Name] = true
			}
			for _, alb := range ar.Albums {
				if (nameMatch || strings.Contains(strings.ToLower(alb.Title), q)) && !seenAlb[alb.Path] {
					albums = append(albums, alb)
					seenAlb[alb.Path] = true
				}
			}
		}
	}
	return
}

func (lib *Library) genreCount() int { return len(lib.Genres) }
func (lib *Library) artistCount() int {
	n := 0
	for _, g := range lib.Genres {
		n += len(g.Artists)
	}
	return n
}
func (lib *Library) albumCount() int {
	n := 0
	for _, g := range lib.Genres {
		for _, ar := range g.Artists {
			n += len(ar.Albums)
		}
	}
	return n
}
func (lib *Library) trackCount() int {
	n := 0
	for _, g := range lib.Genres {
		for _, ar := range g.Artists {
			for _, alb := range ar.Albums {
				n += len(alb.Tracks)
			}
		}
	}
	return n
}
