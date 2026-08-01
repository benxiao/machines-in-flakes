package main

import (
	"html/template"
	"time"
)

// ---- Data structs ----

type BrowsePage struct {
	ActiveTab     string
	IsAdmin       bool
	Paths         []PathRow  // sidebar
	CurrentRoot   string     // abs path of active sidebar entry
	Dir           string
	DirName       string
	Breadcrumbs   []Breadcrumb
	Subdirs       []SubdirRow
	Files         []FileRow
	Playlists     []PlaylistRow
	PlaylistsJSON template.JS
	DirAlbumArt   string
	SortBy        string
	InArchive     bool // browsing inside an archive (zip/rar): read-only, no upload/mutations
}

type PlaylistRow struct {
	ID        int64
	Name      string
	ItemCount int
}

type PlaylistItem struct {
	ID         int64
	Path       string
	Name       string
	FileType   string
	WatchCount int64
}

type FavoriteItem struct {
	Path       string
	Name       string
	Dir        string
	IsFolder   bool
	StartIdx   int
	EndIdx     int
	TrackCount int
}

type FolderPlayPage struct {
	ActiveTab string
	IsAdmin   bool
	Folder    string
	Dir       string
	StartIdx  int
	Items     []PlaylistItem
}

type PlaylistState struct {
	CurrentIndex int
	PositionSec  float64
}

type PlaylistsPage struct {
	ActiveTab string
	IsAdmin   bool
	Playlists []PlaylistRow
	Error     string
}

type PlaylistDetailPage struct {
	ActiveTab string
	IsAdmin   bool
	ID        int64
	Name      string
	Items     []PlaylistItem
	State     PlaylistState
}

type Breadcrumb struct {
	Name    string
	Path    string
	Current bool
}

type SubdirRow struct {
	AbsPath    string
	Name       string
	AlbumArt   string // abs path of cover image inside this dir, empty if none
	ModifiedAt string
	ModTime    time.Time
}

type FileRow struct {
	AbsPath    string
	Filename   string
	Extension  string
	FileType   string
	SizeBytes  int64
	Size       string
	ModifiedAt string
	WatchCount int64
	ModTime    time.Time
	AlbumArt   string // for archives: virtual path of a cover image inside the zip/rar, empty if none
}

type GrantedUserRow struct {
	UserID   int64
	Username string
}

type AdminPathRow struct {
	ID      int64
	Path    string
	Granted bool
}

type UserDetailPage struct {
	ActiveTab   string
	IsAdmin     bool
	ID          int64
	Username    string
	GoogleEmail string
	AllPaths    []AdminPathRow
	Error       string
}

type PathsPage struct {
	ActiveTab string
	Paths     []PathRow
	Error     string
}

type RecentItem struct {
	Path         string
	Filename     string
	FileType     string
	Dir          string
	WatchCount   int64
	UpdatedAt    string
	AddedAt      string // pre-formatted mtime, "Recently added" section only
	PositionSec  float64
	AlbumArt     string
	Context      string // "playlist", "favorites", or "" (plain folder)
	ContextID    int64
	ContextStart int
}

type RecentPage struct {
	ActiveTab  string
	IsAdmin    bool
	Continuing []RecentItem // in-progress items (saved mid-file position), newest played first
	Added      []RecentItem // newest files in the library by filesystem mtime
}

type StatCell struct {
	Class string // hm0 = no play, hmv1..4 / hma1..4 = intensity with dominant hue, hmx = future
	Title string // e.g. "Mon 2 Jan 2026 — video 1h 20m · audio 45m"
}

type StatWeek struct {
	Cells [7]StatCell // Monday..Sunday
	Month string      // month label, set on the first week whose Monday enters a new month
}

type StatsTotals struct {
	TodayVideo, TodayAudio int64
	WeekVideo, WeekAudio   int64
	MonthVideo, MonthAudio int64
	AllVideo, AllAudio     int64
}

type StatsPage struct {
	ActiveTab  string
	IsAdmin    bool
	Weeks      []StatWeek
	HasPlay    bool // any nonzero day in the chart window
	Totals     StatsTotals
	TopItems   []PlaylistItem
	RecentDone []RecentItem
	TopFolders []FolderStat
}

type FolderStat struct {
	Folder    string
	MediaType string
	Seconds   int64
	Pct       int // bar width as % of the largest folder shown
}

type FavoritesPage struct {
	ActiveTab string
	IsAdmin   bool
	Items     []FavoriteItem
	Tracks    []PlaylistItem
}

type PathRow struct {
	ID           int64
	Path         string
	Enabled      bool
	SizeGB       float64
	GrantedUsers []GrantedUserRow
}

type SettingsPage struct {
	ActiveTab string
	IsAdmin   bool
	Paths     []PathRow
	PathError string
}

type LoginPage struct {
	Error string
	Next  string
}

type UsersPage struct {
	ActiveTab  string
	IsAdmin    bool
	Users      []UserRow
	CurrentUID int64
	Error      string
}

type UserRow struct {
	ID        int64
	Username  string
	CreatedAt string
}

type TrashPage struct {
	ActiveTab string
	IsAdmin   bool
	Items     []TrashItemRow
	Error     string
}

type TrashItemRow struct {
	ID           int64
	Name         string
	OriginalPath string
	IsFolder     bool
	DeletedAt    string
}

type DuplicatesPage struct {
	ActiveTab string
	IsAdmin   bool
}
