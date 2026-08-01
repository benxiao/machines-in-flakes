package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---- Stats page ----

func (a *App) handleStatsPage(w http.ResponseWriter, r *http.Request) {
	userID := uid(r)
	ctx := r.Context()

	// GitHub-style activity heatmap: 53 week columns ending on the current
	// week, Monday-start rows. The window begins on the Monday on/before
	// one year ago so every column is a complete calendar week.
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := today.AddDate(0, 0, -364)
	for start.Weekday() != time.Monday {
		start = start.AddDate(0, 0, -1)
	}

	type dayPlay struct{ video, audio int64 }
	byDay := map[string]*dayPlay{}
	rows, err := a.db.Query(ctx, `
		SELECT to_char(day, 'YYYY-MM-DD'), media_type, SUM(seconds)
		FROM play_time
		WHERE user_id = $1 AND day >= $2
		GROUP BY day, media_type
	`, userID, start.Format("2006-01-02"))
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	for rows.Next() {
		var day, mt string
		var sec int64
		if rows.Scan(&day, &mt, &sec) != nil {
			continue
		}
		d := byDay[day]
		if d == nil {
			d = &dayPlay{}
			byDay[day] = d
		}
		if mt == "audio" {
			d.audio += sec
		} else {
			d.video += sec
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpErr(w, err, 500)
		return
	}

	var weeks []StatWeek
	hasPlay := false
	prevMonth := time.Month(0)
	for d := start; !d.After(today); d = d.AddDate(0, 0, 7) {
		wk := StatWeek{}
		if d.Month() != prevMonth {
			wk.Month = d.Format("Jan")
			prevMonth = d.Month()
		}
		for i := range 7 {
			day := d.AddDate(0, 0, i)
			if day.After(today) {
				wk.Cells[i] = StatCell{Class: "hmx"}
				continue
			}
			var video, audio int64
			if v := byDay[day.Format("2006-01-02")]; v != nil {
				video, audio = v.video, v.audio
			}
			tot := video + audio
			cell := StatCell{Class: "hm0", Title: day.Format("Mon 2 Jan 2006") + " — no play time"}
			if tot > 0 {
				hasPlay = true
				lvl := 1
				switch {
				case tot >= 3*3600:
					lvl = 4
				case tot >= 90*60:
					lvl = 3
				case tot >= 30*60:
					lvl = 2
				}
				hue := "v"
				if audio > video {
					hue = "a"
				}
				cell.Class = fmt.Sprintf("hm%s%d", hue, lvl)
				var parts []string
				if video > 0 {
					parts = append(parts, "video "+fmtDurStr(video))
				}
				if audio > 0 {
					parts = append(parts, "audio "+fmtDurStr(audio))
				}
				cell.Title = day.Format("Mon 2 Jan 2006") + " — " + strings.Join(parts, " · ")
			}
			wk.Cells[i] = cell
		}
		weeks = append(weeks, wk)
	}
	// The window can open mid-month, putting the first label a week or two
	// before the next month's; drop it rather than let them overlap.
	for i := range weeks {
		if weeks[i].Month == "" {
			continue
		}
		for j := i + 1; j <= i+2 && j < len(weeks); j++ {
			if weeks[j].Month != "" {
				weeks[i].Month = ""
			}
		}
	}

	var t StatsTotals
	a.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(seconds) FILTER (WHERE day = CURRENT_DATE AND media_type = 'video'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE day = CURRENT_DATE AND media_type = 'audio'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE day >= CURRENT_DATE - 6 AND media_type = 'video'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE day >= CURRENT_DATE - 6 AND media_type = 'audio'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE day >= CURRENT_DATE - 29 AND media_type = 'video'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE day >= CURRENT_DATE - 29 AND media_type = 'audio'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE media_type = 'video'), 0),
			COALESCE(SUM(seconds) FILTER (WHERE media_type = 'audio'), 0)
		FROM play_time WHERE user_id = $1
	`, userID).Scan(
		&t.TodayVideo, &t.TodayAudio, &t.WeekVideo, &t.WeekAudio,
		&t.MonthVideo, &t.MonthAudio, &t.AllVideo, &t.AllAudio,
	)

	var top []PlaylistItem
	if rows, err := a.db.Query(ctx, `
		SELECT fi.path, fi.filename, fi.file_type, vp.watch_count
		FROM file_index fi
		JOIN video_positions vp ON vp.user_id = fi.user_id AND vp.path = fi.path
		WHERE fi.user_id = $1 AND vp.watch_count > 0
		ORDER BY vp.watch_count DESC
		LIMIT 20
	`, userID); err == nil {
		for rows.Next() {
			var it PlaylistItem
			if rows.Scan(&it.Path, &it.Name, &it.FileType, &it.WatchCount) == nil {
				top = append(top, it)
			}
		}
		rows.Close()
	}

	var done []RecentItem
	if rows, err := a.db.Query(ctx, `
		SELECT vp.path, fi.filename, fi.file_type, fi.dir_path, vp.watch_count, vp.updated_at
		FROM video_positions vp
		JOIN file_index fi ON fi.user_id = vp.user_id AND fi.path = vp.path
		WHERE vp.user_id = $1 AND vp.watch_count > 0
		ORDER BY vp.updated_at DESC
		LIMIT 15
	`, userID); err == nil {
		for rows.Next() {
			var it RecentItem
			var at time.Time
			if rows.Scan(&it.Path, &it.Filename, &it.FileType, &it.Dir, &it.WatchCount, &at) == nil {
				it.UpdatedAt = at.Local().Format("2006-01-02 15:04")
				done = append(done, it)
			}
		}
		rows.Close()
	}

	var folders []FolderStat
	if rows, err := a.db.Query(ctx, `
		SELECT media_type, folder, seconds FROM folder_play_time
		WHERE user_id = $1 ORDER BY seconds DESC LIMIT 12
	`, userID); err == nil {
		for rows.Next() {
			var f FolderStat
			if rows.Scan(&f.MediaType, &f.Folder, &f.Seconds) == nil {
				folders = append(folders, f)
			}
		}
		rows.Close()
	}
	if len(folders) > 0 {
		max := folders[0].Seconds
		for i := range folders {
			folders[i].Pct = int(folders[i].Seconds * 100 / max)
			if folders[i].Pct == 0 {
				folders[i].Pct = 1
			}
		}
	}

	render(w, "stats", StatsPage{
		ActiveTab: "stats", IsAdmin: isAdmin(r),
		Weeks: weeks, HasPlay: hasPlay, Totals: t,
		TopItems: top, RecentDone: done, TopFolders: folders,
	})
}
