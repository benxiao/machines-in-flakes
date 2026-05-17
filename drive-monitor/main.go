// Drive health monitoring web server. Runs on port 10090.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const listenPort = 10090

// ── Brand detection ───────────────────────────────────────────────────────────

var brandPrefixes = [][2]string{
	{"ST", "Seagate"}, {"WD", "WD"}, {"SHPP", "SK Hynix"}, {"HFS", "SK Hynix"},
	{"BC5", "SK Hynix"}, {"MZ", "Samsung"}, {"CT", "Crucial"}, {"MTFD", "Micron"},
	{"SSDSC", "Intel"}, {"SSDPE", "Intel"}, {"KBG", "Kioxia"}, {"THNS", "Toshiba"},
	{"MQ", "Toshiba"}, {"HUS", "HGST"}, {"HUC", "HGST"},
}

var knownBrands = []string{
	"Seagate", "Western Digital", "Samsung", "Crucial", "Intel",
	"Toshiba", "HGST", "Hitachi", "SK hynix", "Micron", "Kioxia", "Timetec",
}

// ── smartctl JSON structs ─────────────────────────────────────────────────────

type attrEntry struct {
	Name string `json:"name"`
	Raw  struct {
		Value int64 `json:"value"`
	} `json:"raw"`
}

type smartJSON struct {
	Device       struct{ Type string `json:"type"` } `json:"device"`
	ModelName    string                               `json:"model_name"`
	ModelFamily  string                               `json:"model_family"`
	RotationRate int                                  `json:"rotation_rate"`
	UserCapacity struct{ Bytes int64 `json:"bytes"` } `json:"user_capacity"`
	Temperature  struct{ Current *int `json:"current"` } `json:"temperature"`
	SmartStatus  struct{ Passed *bool `json:"passed"` }  `json:"smart_status"`
	PowerOnTime  struct{ Hours int `json:"hours"` }      `json:"power_on_time"`
	NVMeHealth   struct {
		AvailableSpare          *int  `json:"available_spare"`
		AvailableSpareThreshold *int  `json:"available_spare_threshold"`
		PercentageUsed          *int  `json:"percentage_used"`
		MediaErrors             int64 `json:"media_errors"`
		NumErrLogEntries        int64 `json:"num_err_log_entries"`
		DataUnitsWritten        int64 `json:"data_units_written"`
	} `json:"nvme_smart_health_information_log"`
	NVMeTestLog struct {
		CurrentOp     struct{ Value int `json:"value"` } `json:"current_self_test_operation"`
		CompletionPct *int                               `json:"current_self_test_completion_percent"`
		Table         []struct {
			Code   struct{ String string `json:"string"` } `json:"self_test_code"`
			Result struct{ Value int `json:"value"` }     `json:"self_test_result"`
			PowerOnHours *int                             `json:"power_on_hours"`
		} `json:"table"`
	} `json:"nvme_self_test_log"`
	ATAAttributes struct {
		Table []attrEntry `json:"table"`
	} `json:"ata_smart_attributes"`
	ATAData struct {
		SelfTest struct {
			Status struct {
				Value            int  `json:"value"`
				RemainingPercent *int `json:"remaining_percent"`
			} `json:"status"`
		} `json:"self_test"`
	} `json:"ata_smart_data"`
	ATATestLog struct {
		Standard struct {
			Table []struct {
				Type   struct{ String string `json:"string"` } `json:"type"`
				Status struct{ String string `json:"string"` } `json:"status"`
				LifetimeHours *int                             `json:"lifetime_hours"`
			} `json:"table"`
		} `json:"standard"`
	} `json:"ata_smart_self_test_log"`
}

// ── Application data structs ──────────────────────────────────────────────────

type DriveInfo struct {
	Dev, Brand, Model, Size, Type string
	CapBytes                      int64
	Temp                          *int
	Health                        *bool
	Hours                         int
	Error                         bool
	// NVMe
	Spare, SpareThreshold, PUsed *int
	MediaErrors, ErrLogEntries   int64
	TBW                          *float64
	// ATA common
	Reallocated, Pending, Uncorrectable *int64
	// HDD
	SpinRetries, CRCErrors, LoadCycles, StartStops *int64
	// SSD
	WearAttr string
	WearVal  *int64
	// Self-test
	TestRunning   bool
	TestRemaining *int
	LastTestType  string
	LastTestOK    *bool
	LastTestHours *int
}

type DiskInfo struct {
	ID, Dev, State    string
	Read, Write, Cksum int
}

type VdevInfo struct {
	Type, Name, State string
	Disks             []DiskInfo
}

type PoolInfo struct {
	Name, Health, Size, Free, Frag, Scan, Errors string
	SizeBytes                                     int64
	UsedPct, Read, Write, Cksum                   int
	Vdevs                                         []VdevInfo
}

type zpoolSt struct {
	Scan, Errors       string
	Read, Write, Cksum int
	Vdevs              []VdevInfo
}

// ── Utility helpers ───────────────────────────────────────────────────────────

func run(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return string(out)
}

func fmtBytes(b int64) string {
	f := float64(b)
	for _, u := range []string{"B", "K", "M", "G", "T"} {
		if f < 1024 {
			return fmt.Sprintf("%.0f%s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.0fP", f)
}

func fmtComma(n int) string {
	if n < 0 {
		return "-" + fmtComma(-n)
	}
	s := strconv.Itoa(n)
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	var b strings.Builder
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func h(s string) string { return html.EscapeString(s) }

func extractBrand(modelName, modelFamily string) string {
	for _, brand := range knownBrands {
		if strings.HasPrefix(strings.ToLower(modelFamily), strings.ToLower(brand)) {
			return brand
		}
		if strings.HasPrefix(strings.ToLower(modelName), strings.ToLower(brand)) {
			return brand
		}
	}
	upper := strings.ToUpper(modelName)
	for _, p := range brandPrefixes {
		if strings.HasPrefix(upper, p[0]) {
			return p[1]
		}
	}
	return ""
}

func getAttrRaw(table []attrEntry, name string) *int64 {
	for _, a := range table {
		if a.Name == name {
			v := a.Raw.Value
			return &v
		}
	}
	return nil
}

var wearAttrNames = []string{
	"Wear_Leveling_Count", "SSD_Life_Left", "Available_Reservd_Space",
	"Media_Wearout_Indicator", "Percent_Lifetime_Remain",
	"Remaining_Lifetime_Perc", "Unused_Rsvd_Blk_Cnt_Tot",
}

func getFirstAttr(table []attrEntry, names ...string) (string, *int64) {
	for _, name := range names {
		if v := getAttrRaw(table, name); v != nil {
			return name, v
		}
	}
	return "", nil
}

func i64ptr(v int64) *int64 { return &v }

// ── Drive parsing ─────────────────────────────────────────────────────────────

func parseDrive(dev string) DriveInfo {
	devShort := strings.TrimPrefix(dev, "/dev/")
	raw := run("smartctl", "--json=c", "-i", "-A", "-H", "-l", "selftest", dev)
	if raw == "" {
		return DriveInfo{Dev: devShort, Error: true}
	}
	var d smartJSON
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return DriveInfo{Dev: devShort, Error: true}
	}

	isNVMe := strings.Contains(strings.ToLower(dev), "nvme") ||
		strings.Contains(strings.ToLower(d.Device.Type), "nvme")
	isSSD := d.RotationRate == 0 && !isNVMe

	dtype := "HDD"
	if isNVMe {
		dtype = "NVMe"
	} else if isSSD {
		dtype = "SSD"
	}

	cap := d.UserCapacity.Bytes
	sizeStr := fmt.Sprintf("%.0fG", float64(cap)/1e9)
	if cap >= int64(1e12) {
		sizeStr = fmt.Sprintf("%.1fT", float64(cap)/1e12)
	}

	model := d.ModelName
	if model == "" {
		model = d.ModelFamily
	}
	if model == "" {
		model = "Unknown"
	}

	info := DriveInfo{
		Dev:      devShort,
		Brand:    extractBrand(d.ModelName, d.ModelFamily),
		Model:    model,
		Size:     sizeStr,
		CapBytes: cap,
		Type:     dtype,
		Temp:     d.Temperature.Current,
		Health:   d.SmartStatus.Passed,
		Hours:    d.PowerOnTime.Hours,
	}

	table := d.ATAAttributes.Table

	if isNVMe {
		nh := d.NVMeHealth
		info.Spare = nh.AvailableSpare
		info.SpareThreshold = nh.AvailableSpareThreshold
		info.PUsed = nh.PercentageUsed
		info.MediaErrors = nh.MediaErrors
		info.ErrLogEntries = nh.NumErrLogEntries
		if nh.DataUnitsWritten > 0 {
			tbw := math.Round(float64(nh.DataUnitsWritten)*512000/1e12*10) / 10
			info.TBW = &tbw
		}
		tl := d.NVMeTestLog
		if tl.CurrentOp.Value != 0 {
			info.TestRunning = true
			info.TestRemaining = tl.CompletionPct
		}
		if len(tl.Table) > 0 {
			e := tl.Table[0]
			info.LastTestType = e.Code.String
			ok := e.Result.Value == 0
			info.LastTestOK = &ok
			info.LastTestHours = e.PowerOnHours
		}
	} else {
		info.Reallocated = getAttrRaw(table, "Reallocated_Sector_Ct")
		info.Pending = getAttrRaw(table, "Current_Pending_Sector")
		info.Uncorrectable = getAttrRaw(table, "Offline_Uncorrectable")
		if dtype == "HDD" {
			info.SpinRetries = getAttrRaw(table, "Spin_Retry_Count")
			info.CRCErrors = getAttrRaw(table, "UDMA_CRC_Error_Count")
			info.LoadCycles = getAttrRaw(table, "Load_Cycle_Count")
			info.StartStops = getAttrRaw(table, "Start_Stop_Count")
		} else {
			name, val := getFirstAttr(table, wearAttrNames...)
			info.WearAttr = name
			info.WearVal = val
		}
		st := d.ATAData.SelfTest.Status
		if st.Value == 15 {
			info.TestRunning = true
			info.TestRemaining = st.RemainingPercent
		}
		if entries := d.ATATestLog.Standard.Table; len(entries) > 0 {
			e := entries[0]
			raw2 := strings.ToLower(e.Type.String)
			if strings.Contains(raw2, "extended") || strings.Contains(raw2, "long") {
				info.LastTestType = "Extended"
			} else {
				info.LastTestType = "Short"
			}
			ok := strings.Contains(strings.ToLower(e.Status.String), "without error")
			info.LastTestOK = &ok
			info.LastTestHours = e.LifetimeHours
		}
	}
	return info
}

func getAllDrives() []DriveInfo {
	scan := run("smartctl", "--scan")
	var drives []DriveInfo
	for _, line := range strings.Split(scan, "\n") {
		parts := strings.Fields(line)
		if len(parts) > 0 {
			drives = append(drives, parseDrive(parts[0]))
		}
	}
	return drives
}

// ── ZFS helpers ───────────────────────────────────────────────────────────────

var (
	reNVMePart = regexp.MustCompile(`^(nvme\d+n\d+)p\d+$`)
	reSATAPart = regexp.MustCompile(`^(sd[a-z]+)\d+$`)
	reVdevType = regexp.MustCompile(`^(mirror|raidz\d?|draid)`)
	rePoolDev  = regexp.MustCompile(`^(sd[a-z]+|nvme\d+n?\d*|hd[a-z]+)$`)
	rePartSufx = regexp.MustCompile(`-part\d+$`)
)

func basedev(dev string) string {
	if m := reNVMePart.FindStringSubmatch(dev); m != nil {
		return m[1]
	}
	if m := reSATAPart.FindStringSubmatch(dev); m != nil {
		return m[1]
	}
	return dev
}

func buildDiskIDMap() map[string]string {
	result := map[string]string{}
	entries, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		return result
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/dev/disk/by-id", entry.Name()))
		if err != nil {
			continue
		}
		dev := basedev(filepath.Base(target))
		name := entry.Name()
		result[name] = dev
		clean := rePartSufx.ReplaceAllString(name, "")
		if _, ok := result[clean]; !ok {
			result[clean] = dev
		}
	}
	return result
}

func getIOStat(pool string) (int64, int64) {
	out := run("zpool", "iostat", "-Hp", pool, "1", "2")
	var last string
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) == pool {
			last = line
		}
	}
	if last == "" {
		return 0, 0
	}
	parts := strings.Split(last, "\t")
	if len(parts) < 7 {
		return 0, 0
	}
	r, _ := strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
	w, _ := strconv.ParseInt(strings.TrimSpace(parts[6]), 10, 64)
	return r, w
}

func zpoolParseErr(parts []string) (r, w, c int) {
	if len(parts) >= 5 {
		r, _ = strconv.Atoi(parts[2])
		w, _ = strconv.Atoi(parts[3])
		c, _ = strconv.Atoi(parts[4])
	}
	return
}

func parseZpoolStatus() map[string]zpoolSt {
	out := run("zpool", "status")
	info := map[string]zpoolSt{}
	var pool string
	inConfig := false

	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "pool:") {
			pool = strings.TrimSpace(strings.TrimPrefix(s, "pool:"))
			info[pool] = zpoolSt{Errors: "No known data errors"}
			inConfig = false
			continue
		}
		if pool == "" || s == "" {
			continue
		}
		if strings.HasPrefix(s, "scan:") {
			st := info[pool]
			st.Scan = strings.TrimSpace(strings.TrimPrefix(s, "scan:"))
			info[pool] = st
		} else if strings.HasPrefix(s, "errors:") {
			st := info[pool]
			st.Errors = strings.TrimSpace(strings.TrimPrefix(s, "errors:"))
			info[pool] = st
			inConfig = false
		} else if s == "config:" {
			inConfig = true
		} else if inConfig && strings.HasPrefix(line, "\t") && !strings.Contains(s, "NAME") {
			rest := line[1:]
			indent := len(rest) - len(strings.TrimLeft(rest, " "))
			parts := strings.Fields(s)
			if len(parts) == 0 {
				continue
			}
			name := parts[0]
			state := ""
			if len(parts) > 1 {
				state = parts[1]
			}
			er, ew, ec := zpoolParseErr(parts)
			st := info[pool]
			switch indent {
			case 0:
				st.Read, st.Write, st.Cksum = er, ew, ec
			case 2:
				var vtype string
				if reVdevType.MatchString(name) {
					vtype = strings.SplitN(name, "-", 2)[0]
				} else {
					vtype = "disk"
				}
				disk := DiskInfo{ID: name, State: state, Read: er, Write: ew, Cksum: ec}
				if vtype == "disk" {
					st.Vdevs = append(st.Vdevs, VdevInfo{
						Type: "disk", Name: name, State: state, Disks: []DiskInfo{disk},
					})
				} else {
					st.Vdevs = append(st.Vdevs, VdevInfo{
						Type: vtype, Name: name, State: state,
					})
				}
			case 4:
				if n := len(st.Vdevs); n > 0 {
					disk := DiskInfo{ID: name, State: state, Read: er, Write: ew, Cksum: ec}
					st.Vdevs[n-1].Disks = append(st.Vdevs[n-1].Disks, disk)
				}
			}
			info[pool] = st
		}
	}
	return info
}

func getZFSPools() []PoolInfo {
	out := run("zpool", "list", "-Hp", "-o", "name,health,size,alloc,free,frag")
	if strings.TrimSpace(out) == "" {
		return nil
	}
	status := parseZpoolStatus()
	diskIDMap := buildDiskIDMap()
	var pools []PoolInfo
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		sizeB, _ := strconv.ParseInt(parts[2], 10, 64)
		allocB, _ := strconv.ParseInt(parts[3], 10, 64)
		freeB, _ := strconv.ParseInt(parts[4], 10, 64)
		frag := strings.TrimSuffix(strings.TrimSpace(parts[5]), "%")
		usedPct := 0
		if sizeB > 0 {
			usedPct = int(float64(allocB) / float64(sizeB) * 100)
		}
		name := parts[0]
		pinfo := status[name]
		vdevs := pinfo.Vdevs
		for vi := range vdevs {
			for di := range vdevs[vi].Disks {
				id := vdevs[vi].Disks[di].ID
				if dev, ok := diskIDMap[id]; ok {
					vdevs[vi].Disks[di].Dev = dev
				} else if rePoolDev.MatchString(id) {
					vdevs[vi].Disks[di].Dev = id
				}
			}
		}
		pools = append(pools, PoolInfo{
			Name: name, Health: parts[1],
			Size: fmtBytes(sizeB), SizeBytes: sizeB,
			Free: fmtBytes(freeB), UsedPct: usedPct, Frag: frag,
			Scan: pinfo.Scan, Errors: pinfo.Errors,
			Read: pinfo.Read, Write: pinfo.Write, Cksum: pinfo.Cksum,
			Vdevs: vdevs,
		})
	}
	return pools
}

// ── HTML building blocks ──────────────────────────────────────────────────────

func healthBadge(passed *bool) string {
	if passed == nil {
		return `<span class="badge warn">? N/A</span>`
	}
	if *passed {
		return `<span class="badge ok">✓ PASS</span>`
	}
	return `<span class="badge fail">✗ FAIL</span>`
}

func poolBadge(state string) string {
	cls := "fail"
	if state == "ONLINE" {
		cls = "ok"
	}
	return `<span class="badge ` + cls + `">` + h(state) + `</span>`
}

func statRow(label, value, cls string) string {
	if cls != "" {
		return fmt.Sprintf(`<tr><td>%s</td><td class="v%s">%s</td></tr>`, label, cls, value)
	}
	return fmt.Sprintf(`<tr><td>%s</td><td>%s</td></tr>`, label, value)
}

func counterRow(label string, val *int64, badIfNonzero bool) string {
	if val == nil {
		return ""
	}
	cls := "ok"
	if *val > 0 {
		if badIfNonzero {
			cls = "bad"
		} else {
			cls = "warn"
		}
	}
	return fmt.Sprintf(`<tr><td>%s</td><td class="v%s">%s</td></tr>`, label, cls, fmtComma(int(*val)))
}

func hoursRow(hours int) string {
	if hours == 0 {
		return ""
	}
	y, rem := hours/8760, hours%8760
	label := fmt.Sprintf("%dd", rem/24)
	if y > 0 {
		label = fmt.Sprintf("%dy %dd", y, rem/24)
	}
	return statRow("On time", fmt.Sprintf("%s h (%s)", fmtComma(hours), label), "")
}

func renderDriveCard(d DriveInfo, poolDevs map[string]bool) string {
	if d.Error {
		return fmt.Sprintf(`<div class="card err-card"><div class="dev">%s</div><div class="model">Error reading SMART data</div></div>`, h(d.Dev))
	}

	tempCls, tempStr := "", "—"
	if d.Temp != nil {
		t := *d.Temp
		tempStr = fmt.Sprintf("%d°C", t)
		if t >= 55 {
			tempCls = "bad"
		} else if t >= 45 {
			tempCls = "warn"
		}
	}

	var rows strings.Builder
	rows.WriteString(statRow("Temp", tempStr, tempCls))

	switch d.Type {
	case "NVMe":
		if d.Spare != nil {
			thresh := 10
			if d.SpareThreshold != nil {
				thresh = *d.SpareThreshold
			}
			spareCls := "ok"
			if *d.Spare <= thresh {
				spareCls = "bad"
			} else if *d.Spare < 30 {
				spareCls = "warn"
			}
			rows.WriteString(statRow("Spare", fmt.Sprintf("%d%% (min %d%%)", *d.Spare, thresh), spareCls))
		}
		if d.PUsed != nil {
			wearCls := "ok"
			if *d.PUsed >= 90 {
				wearCls = "bad"
			} else if *d.PUsed >= 50 {
				wearCls = "warn"
			}
			rows.WriteString(statRow("Wear used", fmt.Sprintf("%d%%", *d.PUsed), wearCls))
		}
		rows.WriteString(counterRow("Media errors", i64ptr(d.MediaErrors), true))
		rows.WriteString(counterRow("Err log", i64ptr(d.ErrLogEntries), true))
		if d.TBW != nil {
			rows.WriteString(statRow("TBW", fmt.Sprintf("%.1f TB", *d.TBW), ""))
		}
	case "HDD":
		rows.WriteString(counterRow("Reallocated", d.Reallocated, true))
		rows.WriteString(counterRow("Pending", d.Pending, true))
		rows.WriteString(counterRow("Uncorr.", d.Uncorrectable, true))
		rows.WriteString(counterRow("Spin retries", d.SpinRetries, false))
		rows.WriteString(counterRow("CRC errors", d.CRCErrors, false))
		if d.LoadCycles != nil {
			rows.WriteString(statRow("Load cycles", fmtComma(int(*d.LoadCycles)), ""))
		}
		if d.StartStops != nil {
			rows.WriteString(statRow("Start/stops", fmtComma(int(*d.StartStops)), ""))
		}
	default: // SSD
		rows.WriteString(counterRow("Reallocated", d.Reallocated, true))
		rows.WriteString(counterRow("Pending", d.Pending, true))
		rows.WriteString(counterRow("Uncorr.", d.Uncorrectable, true))
		if d.WearVal != nil {
			rows.WriteString(statRow("Wear left", fmt.Sprintf("%d%%", *d.WearVal), ""))
		}
	}
	rows.WriteString(hoursRow(d.Hours))

	cardCls := "card"
	if d.Health != nil && !*d.Health {
		cardCls = "card fail-card"
	}
	inPool := poolDevs == nil || poolDevs[d.Dev]
	unpoolBadge := ""
	if !inPool {
		unpoolBadge = `<span class="tbadge t-unpool">NO POOL</span>`
	}

	var statusHTML string
	if d.TestRunning {
		remStr := ""
		if d.TestRemaining != nil {
			remStr = fmt.Sprintf(" %d%% left", *d.TestRemaining)
		}
		statusHTML = fmt.Sprintf(`<span class="test-status running">⏳ Test running…%s</span>`, h(remStr))
	} else if d.LastTestType != "" {
		hrsStr := ""
		if d.LastTestHours != nil {
			hrsStr = fmt.Sprintf(" @ %s h", fmtComma(*d.LastTestHours))
		}
		if d.LastTestOK == nil {
			statusHTML = fmt.Sprintf(`<span class="test-status">%s%s</span>`, h(d.LastTestType), h(hrsStr))
		} else if *d.LastTestOK {
			statusHTML = fmt.Sprintf(`<span class="test-status ok">✓ %s OK%s</span>`, h(d.LastTestType), h(hrsStr))
		} else {
			statusHTML = fmt.Sprintf(`<span class="test-status fail">✗ %s FAIL%s</span>`, h(d.LastTestType), h(hrsStr))
		}
	}

	disabled, devJS := "", h(d.Dev)
	if d.TestRunning {
		disabled = " disabled"
	}
	testHTML := fmt.Sprintf(
		`<div class="card-tests">%s<div class="test-btns">`+
			`<button class="test-btn"%s onclick="startTest(this,'%s','short')">Short</button>`+
			`<button class="test-btn"%s onclick="startTest(this,'%s','long')">Long</button>`+
			`</div></div>`,
		statusHTML, disabled, devJS, disabled, devJS,
	)

	brandHTML := ""
	if d.Brand != "" {
		brandHTML = fmt.Sprintf(`<div class="brand">%s</div>`, h(d.Brand))
	}
	model := d.Model
	if len([]rune(model)) > 28 {
		model = string([]rune(model)[:28])
	}
	dtype := strings.ToLower(d.Type)

	return fmt.Sprintf(`<div class="%s">
  <div class="card-head"><span class="dev">%s</span><span class="tbadge t-%s">%s</span>%s</div>
  %s<div class="model" title="%s">%s</div>
  <div class="drsize">%s</div>
  <div class="hrow">%s</div>
  <table class="stats">%s</table>
  %s
</div>`,
		cardCls, h(d.Dev), dtype, d.Type, unpoolBadge,
		brandHTML, h(d.Model), h(model),
		d.Size, healthBadge(d.Health),
		rows.String(), testHTML,
	)
}

func ecSpan(val int, label string) string {
	cls := "ec-ok"
	if val > 0 {
		cls = "ec-bad"
	}
	return fmt.Sprintf(`<span class="ec %s"><span class="ec-lbl">%s</span>%d</span>`, cls, label, val)
}

func renderPoolCard(p PoolInfo, driveByDev map[string]DriveInfo) string {
	u := p.UsedPct
	barCol := "#2ecc71"
	if u > 90 {
		barCol = "#e74c3c"
	} else if u > 75 {
		barCol = "#f39c12"
	}
	borderCol := "#238636"
	if p.Health != "ONLINE" {
		borderCol = "#da3633"
	} else if p.Read > 0 || p.Write > 0 || p.Cksum > 0 {
		borderCol = "#f39c12"
	}
	errsHTML := ecSpan(p.Read, "R") + ecSpan(p.Write, "W") + ecSpan(p.Cksum, "CK")

	var vdevsHTML strings.Builder
	for _, vdev := range p.Vdevs {
		var diskRowsHTML strings.Builder
		for _, disk := range vdev.Disks {
			dotCls := "dok"
			if disk.State != "ONLINE" {
				dotCls = "dfail"
			}
			dr := driveByDev[disk.Dev]
			brand := ""
			if dr.Brand != "" {
				brand = fmt.Sprintf(`<span class="dbrand">%s</span>`, h(dr.Brand))
			}
			model := dr.Model
			if len([]rune(model)) > 30 {
				model = string([]rune(model)[:30])
			}
			devLabel := disk.Dev
			if devLabel == "" {
				devLabel = disk.ID
				if len(devLabel) > 20 {
					devLabel = devLabel[:20]
				}
			}
			derrCls := "disk-errs"
			if disk.Read > 0 || disk.Write > 0 || disk.Cksum > 0 {
				derrCls = "disk-errs bad"
			}
			diskRowsHTML.WriteString(fmt.Sprintf(
				`<div class="disk-row"><span class="ddot %s">●</span>`+
					`<span class="ddev">%s</span>%s<span class="dmodel">%s</span>`+
					`<span class="%s">R:%d W:%d CK:%d</span></div>`,
				dotCls, h(devLabel), brand, h(model), derrCls,
				disk.Read, disk.Write, disk.Cksum,
			))
		}
		vtype := strings.ToUpper(vdev.Type)
		if vdev.Type == "disk" {
			vdevsHTML.WriteString(fmt.Sprintf(
				`<div class="vdev-single"><span class="vtype">SINGLE</span><div class="vdev-disks">%s</div></div>`,
				diskRowsHTML.String(),
			))
		} else {
			vdevsHTML.WriteString(fmt.Sprintf(
				`<div class="vdev"><span class="vtype">%s</span><div class="vdev-disks">%s</div></div>`,
				h(vtype), diskRowsHTML.String(),
			))
		}
	}

	scrubCls := "pc-scrub"
	if p.Errors != "No known data errors" {
		scrubCls = "pc-scrub pc-scrub-err"
	}
	scrubText := p.Scan
	if scrubText == "" {
		scrubText = "no scrub recorded"
	}

	return fmt.Sprintf(`<div class="pool-card" style="border-left-color:%s">
  <div class="pc-head">
    <a class="pname" href="/pool/%s">%s</a>
    <span class="pc-right">
      <span class="psize">%s</span><span class="pdot">·</span>
      <span class="pfree">%s free</span><span class="pdot">·</span>
      <span class="pfrag">frag %s%%</span><span class="pdot">·</span>
      <div class="pc-errs">%s</div>%s
    </span>
  </div>
  <div class="pc-bar-wrap">
    <div class="pc-bar"><div class="pc-bfill" style="width:%d%%;background:%s"></div></div>
    <span class="pc-bpct">%d%%</span>
  </div>
  <div class="pc-vdevs">%s</div>
  <div class="pc-footer">
    <span class="%s">%s</span>
    <button class="scrub-btn" onclick="startScrub(this,'%s')">▶ Scrub</button>
  </div>
</div>`,
		borderCol,
		h(p.Name), h(p.Name),
		p.Size, p.Free, h(p.Frag),
		errsHTML, poolBadge(p.Health),
		u, barCol, u,
		vdevsHTML.String(),
		scrubCls, h(scrubText),
		h(p.Name),
	)
}

// ── Main page CSS ─────────────────────────────────────────────────────────────

const mainCSS = `
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#c9d1d9;min-height:100vh}
header{background:#161b22;border-bottom:1px solid #30363d;padding:14px 24px;display:flex;align-items:center;gap:12px}
.hname{font-size:1.2rem;font-weight:700;color:#58a6ff}
.htitle{color:#8b949e}
.hts{font-size:.82rem;color:#6e7681;margin-left:auto}
.fbanner{background:#3d1515;border-bottom:1px solid #da3633;color:#ff7b7b;padding:8px 24px;font-size:.88rem}
.tabbar{background:#161b22;border-bottom:1px solid #30363d;display:flex;padding:0 24px;gap:4px}
.tab{padding:10px 16px;font-size:.82rem;font-weight:600;border:none;background:none;color:#8b949e;cursor:pointer;border-bottom:2px solid transparent;transition:color .15s}
.tab.active{color:#e6edf3;border-bottom-color:#58a6ff}
.tab:hover:not(.active){color:#c9d1d9}
.tabpanel{padding:20px 24px}
.tabpanel.hidden{display:none}
h2{font-size:.72rem;text-transform:uppercase;letter-spacing:.1em;color:#6e7681;margin-bottom:14px}
.cards{display:flex;flex-wrap:wrap;gap:12px}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:14px;width:190px}
.card.fail-card{border-color:#da3633;background:#1a1015}
.card.err-card{border-color:#9e6a03;opacity:.7}
.card-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:3px}
.dev{font-size:.95rem;font-weight:700;color:#e6edf3}
.brand{font-size:.68rem;font-weight:600;color:#58a6ff;margin-bottom:1px;text-transform:uppercase;letter-spacing:.04em}
.model{font-size:.73rem;color:#8b949e;margin-bottom:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.drsize{font-size:.7rem;color:#6e7681;margin-bottom:8px}
.hrow{margin-bottom:8px}
.badge{display:inline-block;font-size:.7rem;font-weight:700;padding:2px 8px;border-radius:10px}
.badge.ok{background:#1a3a1a;color:#3fb950;border:1px solid #238636}
.badge.fail{background:#3a1a1a;color:#f85149;border:1px solid #da3633}
.badge.warn{background:#3a2e1a;color:#d29922;border:1px solid #9e6a03}
.tbadge{font-size:.62rem;font-weight:800;padding:2px 5px;border-radius:4px}
.t-nvme{background:#1a2d4a;color:#58a6ff}
.t-ssd{background:#1a3a2a;color:#3fb950}
.t-hdd{background:#2e2a1a;color:#d29922}
.t-unpool{background:#2e1f3a;color:#bc8cff;border:1px solid #6e40c9}
.stats{width:100%;border-collapse:collapse;font-size:.73rem}
.stats td{padding:2px 0}
.stats td:first-child{color:#8b949e;width:76px}
.stats td:last-child{color:#c9d1d9;font-variant-numeric:tabular-nums}
.vok{color:#3fb950!important}.vwarn{color:#d29922!important}
.vbad{color:#f85149!important;font-weight:700}
.pool-list{display:grid;grid-template-columns:repeat(auto-fill,minmax(460px,1fr));gap:14px}
.pool-card{background:#161b22;border:1px solid #30363d;border-left:3px solid #238636;border-radius:8px;padding:16px}
.pc-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}
.pname{font-size:1.05rem;font-weight:700;color:#e6edf3;text-decoration:none}
.pname:hover{color:#58a6ff;text-decoration:underline}
.pc-right{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.psize{font-size:.78rem;color:#8b949e}
.pfree{font-size:.78rem;color:#3fb950}
.pfrag{font-size:.78rem;color:#6e7681}
.pdot{color:#30363d;font-size:.8rem}
.pc-bar-wrap{display:flex;align-items:center;gap:8px;margin-bottom:12px}
.pc-bar{flex:1;background:#21262d;border-radius:6px;height:10px;overflow:hidden}
.pc-bfill{height:100%;border-radius:6px;transition:width .3s}
.pc-bpct{font-size:.72rem;font-weight:700;color:#c9d1d9;min-width:30px;text-align:right;font-variant-numeric:tabular-nums}
.pc-errs{display:flex;gap:4px}
.ec{display:inline-flex;align-items:center;gap:3px;font-size:.68rem;font-variant-numeric:tabular-nums;padding:1px 6px;border-radius:10px}
.ec-lbl{font-weight:700;opacity:.7}
.ec-ok{background:#1a2a1a;color:#8b949e;border:1px solid #21262d}
.ec-bad{background:#3a1a1a;color:#f85149;border:1px solid #da3633;font-weight:700}
.pc-vdevs{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:12px}
.vdev{background:#0d1117;border:1px solid #21262d;border-radius:6px;padding:10px 12px;flex:1;min-width:200px}
.vdev-single{background:#0d1117;border:1px solid #21262d;border-radius:6px;padding:10px 12px;flex:1}
.vtype{display:inline-block;font-size:.6rem;font-weight:800;text-transform:uppercase;letter-spacing:.1em;color:#58a6ff;background:#1a2d4a;padding:1px 6px;border-radius:4px;margin-bottom:8px}
.vdev-disks{display:flex;flex-direction:column;gap:5px}
.disk-row{display:flex;align-items:center;gap:7px;font-size:.75rem}
.ddot{font-size:.55rem;flex-shrink:0}
.dok{color:#3fb950}.dfail{color:#f85149}
.ddev{font-weight:700;color:#e6edf3;min-width:62px;flex-shrink:0}
.dbrand{color:#58a6ff;font-size:.67rem;font-weight:600;min-width:44px;flex-shrink:0}
.dmodel{color:#8b949e;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
.disk-errs{font-size:.67rem;color:#6e7681;font-variant-numeric:tabular-nums;flex-shrink:0;white-space:nowrap}
.disk-errs.bad{color:#f85149;font-weight:700}
.pc-footer{border-top:1px solid #21262d;padding-top:10px;display:flex;align-items:center;justify-content:space-between;gap:10px}
.pc-scrub{font-size:.73rem;color:#6e7681;flex:1}
.pc-scrub-err{color:#f85149}
.scrub-btn{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #238636;background:#1a3a1a;color:#3fb950;cursor:pointer;white-space:nowrap;flex-shrink:0}
.scrub-btn:hover{background:#238636;color:#fff}
.scrub-btn:disabled{opacity:.5;cursor:default}
.io-toolbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px}
.io-toolbar-legend{display:flex;gap:16px;font-size:.75rem}
.io-leg-dot{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:4px}
.win-btns{display:flex;gap:6px}
.win-btn{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer}
.win-btn:hover{border-color:#58a6ff;color:#58a6ff}
.win-btn.active{border-color:#58a6ff;background:#1a2d4a;color:#58a6ff}
.io-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(420px,1fr));gap:14px}
.io-card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:14px}
.io-card-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px}
.io-pname{font-size:.95rem;font-weight:700;color:#e6edf3;text-decoration:none}
.io-pname:hover{color:#58a6ff}
.io-speeds{display:flex;gap:20px;margin-bottom:8px;font-size:.82rem;font-variant-numeric:tabular-nums}
.io-read{color:#3fb950;font-weight:600}
.io-write{color:#f0883e;font-weight:600}
.io-lbl{color:#6e7681;font-size:.7rem;margin-right:4px}
canvas.io-canvas{width:100%;height:140px;display:block;border-radius:4px;background:#0d1117}
.card-tests{border-top:1px solid #21262d;margin-top:8px;padding-top:7px;display:flex;flex-direction:column;gap:5px}
.test-status{font-size:.68rem;color:#6e7681}
.test-status.running{color:#d29922}
.test-status.ok{color:#3fb950}
.test-status.fail{color:#f85149;font-weight:700}
.test-btns{display:flex;gap:5px}
.test-btn{font-size:.68rem;font-weight:700;padding:2px 8px;border-radius:5px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer;flex:1}
.test-btn:hover:not(:disabled){border-color:#58a6ff;color:#58a6ff}
.test-btn:disabled{opacity:.45;cursor:default}
@media (max-width:640px){
  header{padding:10px 16px;gap:8px}
  .hts{display:none}
  .tabbar{padding:0 8px;overflow-x:auto;-webkit-overflow-scrolling:touch}
  .tab{padding:8px 10px;font-size:.76rem;white-space:nowrap}
  .tabpanel{padding:14px 16px}
  .card{width:100%;max-width:100%}
  .pool-list{grid-template-columns:1fr}
  .io-grid{grid-template-columns:1fr}
  .pc-head{flex-direction:column;align-items:flex-start;gap:4px}
  .io-toolbar{flex-direction:column;gap:8px;align-items:flex-start}
  .io-card-head{flex-direction:column;align-items:flex-start;gap:4px}
  .scrub-btn,.win-btn{min-height:36px;padding:6px 12px}
  .test-btn{min-height:36px;padding:6px 8px}
}
`

// ── Pool detail CSS ───────────────────────────────────────────────────────────

const poolDetailCSS = `
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#c9d1d9;min-height:100vh}
header{background:#161b22;border-bottom:1px solid #30363d;padding:14px 24px;display:flex;align-items:center;gap:14px}
.back{color:#58a6ff;text-decoration:none;font-size:.85rem}
.back:hover{text-decoration:underline}
.htitle{font-size:1.1rem;font-weight:700;color:#e6edf3}
.subtitle{font-size:.8rem;color:#6e7681}
section{padding:24px}
.speeds{display:flex;gap:32px;margin-bottom:20px}
.speed-box{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px 24px;min-width:160px}
.speed-lbl{font-size:.7rem;text-transform:uppercase;letter-spacing:.08em;color:#8b949e;margin-bottom:6px}
.speed-val{font-size:1.8rem;font-weight:700;font-variant-numeric:tabular-nums}
.speed-read{color:#3fb950}
.speed-write{color:#f0883e}
.graph-wrap{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px}
.graph-toolbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}
.legend{display:flex;gap:20px;font-size:.75rem}
.leg-dot{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:5px}
.win-btns{display:flex;gap:6px}
.win-btn{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer}
.win-btn:hover{border-color:#58a6ff;color:#58a6ff}
.win-btn.active{border-color:#58a6ff;background:#1a2d4a;color:#58a6ff}
canvas{width:100%;height:300px;display:block}
@media (max-width:640px){
  header{padding:10px 16px}
  section{padding:14px 16px}
  .speeds{flex-wrap:wrap;gap:12px}
  .speed-box{flex:1 1 120px}
  .graph-toolbar{flex-direction:column;gap:8px;align-items:flex-start}
  .win-btn{min-height:36px;padding:6px 12px}
}
`

// ── Full page renderers ───────────────────────────────────────────────────────

func renderPage(drives []DriveInfo, pools []PoolInfo) string {
	hostname, _ := os.Hostname()
	now := time.Now().Format("2006-01-02 15:04:05")
	failCount := 0
	for _, d := range drives {
		if d.Health != nil && !*d.Health {
			failCount++
		}
	}

	driveByDev := map[string]DriveInfo{}
	for _, d := range drives {
		if !d.Error {
			driveByDev[d.Dev] = d
		}
	}

	poolDevs := map[string]bool{}
	for _, p := range pools {
		for _, vdev := range p.Vdevs {
			for _, disk := range vdev.Disks {
				if disk.Dev != "" {
					poolDevs[disk.Dev] = true
					// nvme0n1 → nvme0
					if m := regexp.MustCompile(`n\d+$`).FindString(disk.Dev); m != "" {
						poolDevs[strings.TrimSuffix(disk.Dev, m)] = true
					}
				}
			}
		}
	}

	var cards strings.Builder
	for _, d := range drives {
		cards.WriteString(renderDriveCard(d, poolDevs))
		cards.WriteByte('\n')
	}

	var poolCards strings.Builder
	for _, p := range pools {
		poolCards.WriteString(renderPoolCard(p, driveByDev))
		poolCards.WriteByte('\n')
	}
	poolHTML := `<p>No ZFS pools found.</p>`
	if len(pools) > 0 {
		poolHTML = `<div class="pool-list">` + poolCards.String() + `</div>`
	}

	banner := ""
	if failCount > 0 {
		banner = fmt.Sprintf(`<div class="fbanner">⚠ %d drive(s) reporting SMART failure</div>`, failCount)
	}

	var poolNames []string
	for _, p := range pools {
		poolNames = append(poolNames, p.Name)
	}
	poolNamesJS, _ := json.Marshal(poolNames)

	totalCapDrives := int64(0)
	for _, d := range drives {
		totalCapDrives += d.CapBytes
	}
	totalCapPools := int64(0)
	for _, p := range pools {
		totalCapPools += p.SizeBytes
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + h(hostname) + ` drives</title>
<style>` + mainCSS + `</style>
</head>
<body>
<header>
  <span class="hname">` + h(hostname) + `</span>
  <span class="htitle">drive health</span>
  <span class="hts">refreshed ` + h(now) + `</span>
</header>
` + banner + `
<div class="tabbar">
  <button class="tab active" onclick="showTab('drives',this)">Drives (` + strconv.Itoa(len(drives)) + `)</button>
  <button class="tab"        onclick="showTab('pools',this)">Pools (` + strconv.Itoa(len(pools)) + `)</button>
  <button class="tab"        onclick="showTab('io',this)">Active I/O</button>
</div>

<div id="tab-drives" class="tabpanel">
  <h2>` + fmtBytes(totalCapDrives) + ` total raw capacity across ` + strconv.Itoa(len(drives)) + ` drives</h2><br>
  <div class="cards">` + cards.String() + `</div>
</div>

<div id="tab-pools" class="tabpanel hidden">
  <h2>` + fmtBytes(totalCapPools) + ` total usable capacity across ` + strconv.Itoa(len(pools)) + ` pools</h2><br>
  ` + poolHTML + `
</div>

<div id="tab-io" class="tabpanel hidden">
  <div class="io-toolbar">
    <div class="io-toolbar-legend">
      <span><span class="io-leg-dot" style="background:#3fb950"></span>Read</span>
      <span><span class="io-leg-dot" style="background:#f0883e"></span>Write</span>
    </div>
    <div class="win-btns">
      <button class="win-btn active" id="io-btn-60"   onclick="setIOWin(60)">1m</button>
      <button class="win-btn"        id="io-btn-300"  onclick="setIOWin(300)">5m</button>
      <button class="win-btn"        id="io-btn-900"  onclick="setIOWin(900)">15m</button>
      <button class="win-btn"        id="io-btn-1800" onclick="setIOWin(1800)">30m</button>
    </div>
  </div>
  <div class="io-grid" id="io-grid"></div>
</div>

<script>
function showTab(name,btn){
  document.querySelectorAll('.tabpanel').forEach(p=>p.classList.add('hidden'));
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  document.getElementById('tab-'+name).classList.remove('hidden');
  btn.classList.add('active');
  if(name==='io')startIO();else stopIO();
}
function startScrub(btn,pool){
  btn.disabled=true;btn.textContent='⏳ Starting…';
  fetch('/scrub/'+pool,{method:'POST'})
    .then(r=>r.ok?(btn.textContent='✓ Started',setTimeout(()=>location.reload(),1500)):(btn.textContent='✗ Error',btn.disabled=false))
    .catch(()=>(btn.textContent='✗ Error',btn.disabled=false));
}
function startTest(btn,dev,kind){
  const btns=btn.parentElement.querySelectorAll('.test-btn');
  btns.forEach(b=>b.disabled=true);btn.textContent='⏳ Starting…';
  fetch('/test/'+dev+'/'+kind,{method:'POST'})
    .then(r=>r.ok?(btn.textContent='✓ Started',setTimeout(()=>location.reload(),2000)):(btn.textContent='✗ Error',btns.forEach(b=>b.disabled=false)))
    .catch(()=>(btn.textContent='✗ Error',btns.forEach(b=>b.disabled=false)));
}
const IO_POOLS=` + string(poolNamesJS) + `;
const IO_BUF=1800,dpr=devicePixelRatio||1;
let ioState={},ioWin=60,ioTick=null;
function fmtMB(b){const mb=b/1048576;return mb>=100?mb.toFixed(0)+' MB/s':mb.toFixed(2)+' MB/s';}
function timeFmt(ts){
  const d=new Date(ts),hh=String(d.getHours()).padStart(2,'0'),mm=String(d.getMinutes()).padStart(2,'0'),ss=String(d.getSeconds()).padStart(2,'0');
  return ioWin<=300?` + "`${hh}:${mm}:${ss}`" + `:` + "`${hh}:${mm}`" + `;
}
function initIO(){
  const grid=document.getElementById('io-grid');
  if(grid.children.length)return;
  IO_POOLS.forEach(pool=>{
    const card=document.createElement('div');card.className='io-card';
    card.innerHTML=` + "`" + `<div class="io-card-head"><a class="io-pname" href="/pool/${pool}">${pool}</a>
      <div class="io-speeds"><span><span class="io-lbl">R</span><span class="io-read" id="io-r-${pool}">—</span></span>
      <span><span class="io-lbl">W</span><span class="io-write" id="io-w-${pool}">—</span></span></div></div>
      <canvas class="io-canvas" id="io-c-${pool}"></canvas>` + "`" + `;
    grid.appendChild(card);
    const canvas=document.getElementById('io-c-'+pool);
    canvas.width=canvas.offsetWidth*dpr;canvas.height=canvas.offsetHeight*dpr;
    ioState[pool]={r:[],w:[],t:[],canvas,ctx:canvas.getContext('2d'),
      rEl:document.getElementById('io-r-'+pool),wEl:document.getElementById('io-w-'+pool)};
  });
}
function startIO(){initIO();if(!ioTick){pollIO();ioTick=setInterval(pollIO,1000);}}
function stopIO(){if(ioTick){clearInterval(ioTick);ioTick=null;}}
function setIOWin(s){
  ioWin=s;
  document.querySelectorAll('.win-btn').forEach(b=>b.classList.remove('active'));
  document.getElementById('io-btn-'+s).classList.add('active');
  IO_POOLS.forEach(drawIO);
}
async function pollIO(){
  await Promise.all(IO_POOLS.map(async pool=>{
    try{
      const d=await fetch('/api/iostat/'+pool).then(r=>r.json());
      const s=ioState[pool];if(!s)return;
      s.r.push(d.read);s.w.push(d.write);s.t.push(Date.now());
      if(s.r.length>IO_BUF){s.r.shift();s.w.shift();s.t.shift();}
      s.rEl.textContent=fmtMB(d.read);s.wEl.textContent=fmtMB(d.write);
      drawIO(pool);
    }catch(e){}
  }));
}
function drawIO(pool){
  const s=ioState[pool];if(!s)return;
  const n=Math.min(s.r.length,ioWin),r=s.r.slice(-n),w=s.w.slice(-n),t=s.t.slice(-n);
  const{canvas,ctx}=s,W=canvas.width,H=canvas.height;
  const PAD_L=44*dpr,PAD_B=20*dpr,GW=W-PAD_L,GH=H-PAD_B;
  ctx.clearRect(0,0,W,H);ctx.fillStyle='#0d1117';ctx.fillRect(0,0,W,H);
  const peak=Math.max(1048576,...r,...w),fs=Math.round(9*dpr);
  ctx.font=fs+'px monospace';
  [0.5,1.0].forEach(f=>{
    const y=PAD_B+GH-GH*f*0.9;
    ctx.strokeStyle='#21262d';ctx.lineWidth=1;ctx.beginPath();ctx.moveTo(PAD_L,y);ctx.lineTo(W,y);ctx.stroke();
    ctx.fillStyle='#6e7681';ctx.textAlign='right';ctx.fillText((peak*f/1048576).toFixed(1),PAD_L-3,y+fs*0.35);
  });
  if(t.length>=2){
    const t0=t[0],t1=t[t.length-1],span=t1-t0||1;
    const tickSecs=ioWin<=60?10:ioWin<=300?30:ioWin<=900?120:300;
    const firstTick=Math.ceil(t0/(tickSecs*1000))*tickSecs*1000;
    ctx.textAlign='center';
    for(let ts=firstTick;ts<=t1;ts+=tickSecs*1000){
      const x=PAD_L+GW*(ts-t0)/span;
      ctx.strokeStyle='#30363d';ctx.lineWidth=1;ctx.beginPath();ctx.moveTo(x,PAD_B);ctx.lineTo(x,PAD_B+GH);ctx.stroke();
      ctx.fillStyle='#6e7681';const lbl=timeFmt(ts),lw=ctx.measureText(lbl).width;
      ctx.fillText(lbl,Math.max(PAD_L+lw/2,Math.min(x,W-lw/2)),H-3);
    }
  }
  function line(data,color){
    if(data.length<2)return;
    ctx.strokeStyle=color;ctx.lineWidth=1.5*dpr;ctx.beginPath();
    data.forEach((v,i)=>{const x=PAD_L+GW*i/(data.length-1),y=PAD_B+GH-(v/peak)*GH*0.9;i===0?ctx.moveTo(x,y):ctx.lineTo(x,y);});
    ctx.stroke();
  }
  line(r,'#3fb950');line(w,'#f0883e');
}
setTimeout(()=>location.reload(),86400000);
</script>
</body>
</html>`
}

func renderPoolDetail(poolName string) string {
	nameEsc := h(poolName)
	poolNameJS, _ := json.Marshal(poolName)
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + nameEsc + ` I/O</title>
<style>` + poolDetailCSS + `</style>
</head>
<body>
<header>
  <a class="back" href="/">← back</a>
  <span class="htitle">` + nameEsc + `</span>
  <span class="subtitle">real-time I/O</span>
</header>
<section>
  <div class="speeds">
    <div class="speed-box"><div class="speed-lbl">Read</div><div class="speed-val speed-read" id="read-val">—</div></div>
    <div class="speed-box"><div class="speed-lbl">Write</div><div class="speed-val speed-write" id="write-val">—</div></div>
  </div>
  <div class="graph-wrap">
    <div class="graph-toolbar">
      <div class="legend">
        <span><span class="leg-dot" style="background:#3fb950"></span>Read</span>
        <span><span class="leg-dot" style="background:#f0883e"></span>Write</span>
      </div>
      <div class="win-btns">
        <button class="win-btn active" id="btn-60"   onclick="setWin(60)">1m</button>
        <button class="win-btn"        id="btn-300"  onclick="setWin(300)">5m</button>
        <button class="win-btn"        id="btn-900"  onclick="setWin(900)">15m</button>
        <button class="win-btn"        id="btn-1800" onclick="setWin(1800)">30m</button>
      </div>
    </div>
    <canvas id="graph"></canvas>
  </div>
</section>
<script>
const pool=` + string(poolNameJS) + `;
const BUFFER=1800;
let rBuf=[],wBuf=[],tBuf=[],winSecs=60;
const dpr=devicePixelRatio||1;
const canvas=document.getElementById('graph');
const ctx=canvas.getContext('2d');
function resize(){canvas.width=canvas.offsetWidth*dpr;canvas.height=canvas.offsetHeight*dpr;draw();}
function setWin(s){
  winSecs=s;
  document.querySelectorAll('.win-btn').forEach(b=>b.classList.remove('active'));
  document.getElementById('btn-'+s).classList.add('active');
  draw();
}
function fmt(b){const mb=b/1048576;return mb>=100?mb.toFixed(0)+' MB/s':mb.toFixed(2)+' MB/s';}
function timeFmt(ts){
  const d=new Date(ts),hh=String(d.getHours()).padStart(2,'0'),mm=String(d.getMinutes()).padStart(2,'0'),ss=String(d.getSeconds()).padStart(2,'0');
  return winSecs<=300?` + "`${hh}:${mm}:${ss}`" + `:` + "`${hh}:${mm}`" + `;
}
function draw(){
  const n=Math.min(rBuf.length,winSecs),r=rBuf.slice(-n),w=wBuf.slice(-n),t=tBuf.slice(-n);
  if(!r.length)return;
  const W=canvas.width,H=canvas.height,PAD_L=52*dpr,PAD_B=26*dpr,GW=W-PAD_L,GH=H-PAD_B;
  ctx.clearRect(0,0,W,H);ctx.fillStyle='#0d1117';ctx.fillRect(0,0,W,H);
  const peak=Math.max(1048576,...r,...w),fs=Math.round(10*dpr);
  ctx.font=fs+'px monospace';
  [0.25,0.5,0.75,1.0].forEach(f=>{
    const y=PAD_B+GH-GH*f*0.92;
    ctx.strokeStyle='#21262d';ctx.lineWidth=1;ctx.beginPath();ctx.moveTo(PAD_L,y);ctx.lineTo(W,y);ctx.stroke();
    ctx.fillStyle='#6e7681';ctx.textAlign='right';
    ctx.fillText((peak*f/1048576).toFixed(f<0.3?2:1),PAD_L-4,y+fs*0.35);
  });
  if(t.length>=2){
    const t0=t[0],t1=t[t.length-1],span=t1-t0||1;
    const tickSecs=winSecs<=60?10:winSecs<=300?30:winSecs<=900?120:300;
    const firstTick=Math.ceil(t0/(tickSecs*1000))*tickSecs*1000;
    ctx.textAlign='center';
    for(let ts=firstTick;ts<=t1;ts+=tickSecs*1000){
      const x=PAD_L+GW*(ts-t0)/span;
      ctx.strokeStyle='#30363d';ctx.lineWidth=1;ctx.beginPath();ctx.moveTo(x,PAD_B);ctx.lineTo(x,PAD_B+GH);ctx.stroke();
      ctx.fillStyle='#6e7681';const lbl=timeFmt(ts),lw=ctx.measureText(lbl).width;
      ctx.fillText(lbl,Math.max(PAD_L+lw/2,Math.min(x,W-lw/2)),H-5);
    }
  }
  function line(data,color){
    if(data.length<2)return;
    ctx.strokeStyle=color;ctx.lineWidth=2*dpr;ctx.beginPath();
    data.forEach((v,i)=>{const x=PAD_L+GW*i/(data.length-1),y=PAD_B+GH-(v/peak)*GH*0.92;i===0?ctx.moveTo(x,y):ctx.lineTo(x,y);});
    ctx.stroke();
  }
  line(r,'#3fb950');line(w,'#f0883e');
}
async function poll(){
  try{
    const d=await fetch('/api/iostat/'+pool).then(r=>r.json());
    rBuf.push(d.read);wBuf.push(d.write);tBuf.push(Date.now());
    if(rBuf.length>BUFFER){rBuf.shift();wBuf.shift();tBuf.shift();}
    document.getElementById('read-val').textContent=fmt(d.read);
    document.getElementById('write-val').textContent=fmt(d.write);
    draw();
  }catch(e){}
}
window.addEventListener('resize',resize);
resize();poll();setInterval(poll,1000);
</script>
</body>
</html>`
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

var (
	rePool    = regexp.MustCompile(`^/pool/([a-zA-Z0-9_-]+)$`)
	reIOStat  = regexp.MustCompile(`^/api/iostat/([a-zA-Z0-9_-]+)$`)
	reScrub   = regexp.MustCompile(`^/scrub/([a-zA-Z0-9_-]+)$`)
	reTest    = regexp.MustCompile(`^/test/([a-zA-Z0-9_-]+)/(short|long)$`)
)

func send(w http.ResponseWriter, code int, body []byte, ct string) {
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(code)
	w.Write(body)
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if r.Method == http.MethodGet {
		switch {
		case path == "/" || path == "/index.html":
			drives, pools := getAllDrives(), getZFSPools()
			send(w, 200, []byte(renderPage(drives, pools)), "text/html; charset=utf-8")

		case rePool.MatchString(path):
			m := rePool.FindStringSubmatch(path)
			send(w, 200, []byte(renderPoolDetail(m[1])), "text/html; charset=utf-8")

		case reIOStat.MatchString(path):
			m := reIOStat.FindStringSubmatch(path)
			rd, wr := getIOStat(m[1])
			body, _ := json.Marshal(map[string]int64{"read": rd, "write": wr})
			send(w, 200, body, "application/json")

		default:
			send(w, 404, []byte("not found"), "text/plain")
		}
		return
	}

	if r.Method == http.MethodPost {
		switch {
		case reScrub.MatchString(path):
			m := reScrub.FindStringSubmatch(path)
			run("zpool", "scrub", m[1])
			send(w, 200, []byte("ok"), "text/plain")

		case reTest.MatchString(path):
			m := reTest.FindStringSubmatch(path)
			run("smartctl", "-t", m[2], "/dev/"+m[1])
			send(w, 200, []byte("ok"), "text/plain")

		default:
			send(w, 400, []byte("bad request"), "text/plain")
		}
		return
	}

	send(w, 405, []byte("method not allowed"), "text/plain")
}

func main() {
	addr := fmt.Sprintf(":%d", listenPort)
	log.Printf("Drive monitor on http://0.0.0.0:%d", listenPort)
	if err := http.ListenAndServe(addr, http.HandlerFunc(handler)); err != nil {
		log.Fatal(err)
	}
}
