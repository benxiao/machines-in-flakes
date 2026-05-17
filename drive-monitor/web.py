#!/usr/bin/env python3
"""Drive health monitoring web server. Runs on port 10090."""
import html as html_mod
import json
import os
import re
import socket
import socketserver
import subprocess
import threading
import http.server
from datetime import datetime

PORT = 10090


def run(*args: str) -> str:
    try:
        r = subprocess.run(list(args), capture_output=True, text=True, timeout=30)
        return r.stdout
    except Exception:
        return ""


def fmt_bytes(b: int) -> str:
    for unit in ("B", "K", "M", "G", "T", "P"):
        if b < 1024:
            return f"{b:.0f}{unit}"
        b //= 1024
    return f"{b}P"


_BRAND_PREFIXES = [
    ("ST",    "Seagate"),
    ("WD",    "WD"),
    ("SHPP",  "SK Hynix"),
    ("HFS",   "SK Hynix"),
    ("BC5",   "SK Hynix"),
    ("MZ",    "Samsung"),
    ("CT",    "Crucial"),
    ("MTFD",  "Micron"),
    ("SSDSC", "Intel"),
    ("SSDPE", "Intel"),
    ("KBG",   "Kioxia"),
    ("THNS",  "Toshiba"),
    ("MQ",    "Toshiba"),
    ("HUS",   "HGST"),
    ("HUC",   "HGST"),
]

_KNOWN_BRANDS = [
    "Seagate", "Western Digital", "Samsung", "Crucial", "Intel",
    "Toshiba", "HGST", "Hitachi", "SK hynix", "Micron", "Kioxia", "Timetec",
]


def extract_brand(model_name: str, model_family: str) -> str:
    for brand in _KNOWN_BRANDS:
        if model_family.lower().startswith(brand.lower()):
            return brand
        if model_name.lower().startswith(brand.lower()):
            return brand
    upper = model_name.upper()
    for prefix, brand in _BRAND_PREFIXES:
        if upper.startswith(prefix):
            return brand
    return ""


def get_attr_raw(table: list, name: str) -> int | None:
    for attr in table or []:
        if attr.get("name") == name:
            raw = attr.get("raw", {})
            return raw.get("value") if isinstance(raw, dict) else raw
    return None


def get_first_attr(table: list, *names: str) -> tuple[str, int] | tuple[None, None]:
    """Return (attribute_name, raw_value) for the first matching name found."""
    for name in names:
        val = get_attr_raw(table, name)
        if val is not None:
            return name, val
    return None, None


def parse_drive(dev: str) -> dict:
    raw = run("smartctl", "--json=c", "-i", "-A", "-H", "-l", "selftest", dev)
    if not raw:
        return {"dev": dev.replace("/dev/", ""), "error": True}
    try:
        d = json.loads(raw)
    except json.JSONDecodeError:
        return {"dev": dev.replace("/dev/", ""), "error": True}

    dev_type = d.get("device", {}).get("type", "")
    is_nvme = "nvme" in dev.lower() or "nvme" in dev_type.lower()
    rotation = d.get("rotation_rate", 1)
    is_ssd = (rotation == 0) and not is_nvme

    if is_nvme:
        dtype = "NVMe"
    elif is_ssd:
        dtype = "SSD"
    else:
        dtype = "HDD"

    cap = d.get("user_capacity", {}).get("bytes", 0)
    size_str = f"{cap / 1e12:.1f}T" if cap >= 1e12 else f"{cap / 1e9:.0f}G"

    model_name = d.get("model_name", "")
    model_family = d.get("model_family", "")

    result: dict = {
        "dev": dev.replace("/dev/", ""),
        "brand": extract_brand(model_name, model_family),
        "model": model_name or model_family or "Unknown",
        "size": size_str,
        "cap_bytes": cap,
        "type": dtype,
        "temp": d.get("temperature", {}).get("current"),
        "health": d.get("smart_status", {}).get("passed"),
        "hours": d.get("power_on_time", {}).get("hours", 0),
        "error": False,
    }

    if is_nvme:
        nvme = d.get("nvme_smart_health_information_log", {})
        result["spare"] = nvme.get("available_spare")
        result["spare_threshold"] = nvme.get("available_spare_threshold")
        result["pused"] = nvme.get("percentage_used")
        result["media_errors"] = nvme.get("media_errors", 0)
        result["err_log_entries"] = nvme.get("num_err_log_entries", 0)
        written = nvme.get("data_units_written", 0)
        result["tbw"] = round(written * 512000 / 1e12, 1) if written else None
    else:
        table = d.get("ata_smart_attributes", {}).get("table", [])
        # Critical for both HDD and SSD — any non-zero is a problem
        result["reallocated"] = get_attr_raw(table, "Reallocated_Sector_Ct")
        result["pending"] = get_attr_raw(table, "Current_Pending_Sector")
        result["uncorrectable"] = get_attr_raw(table, "Offline_Uncorrectable")

        if dtype == "HDD":
            # Mechanical health indicators
            result["spin_retries"] = get_attr_raw(table, "Spin_Retry_Count")
            result["crc_errors"] = get_attr_raw(table, "UDMA_CRC_Error_Count")
            result["load_cycles"] = get_attr_raw(table, "Load_Cycle_Count")
            result["start_stops"] = get_attr_raw(table, "Start_Stop_Count")
        else:
            # SSD wear — manufacturers use different attribute names
            wear_name, wear_val = get_first_attr(
                table,
                "Wear_Leveling_Count",       # Samsung (177) — value = % remaining
                "SSD_Life_Left",             # Intel (231)
                "Available_Reservd_Space",   # Intel (232)
                "Media_Wearout_Indicator",   # Intel (233)
                "Percent_Lifetime_Remain",   # Crucial/Micron (202)
                "Remaining_Lifetime_Perc",   # OCZ/Toshiba
                "Unused_Rsvd_Blk_Cnt_Tot",  # WD
            )
            result["wear_attr"] = wear_name
            result["wear_val"] = wear_val

    # Self-test status
    result["test_running"] = False
    result["test_remaining"] = None
    result["last_test_type"] = None
    result["last_test_ok"] = None
    result["last_test_hours"] = None

    if is_nvme:
        nvme_log = d.get("nvme_self_test_log", {})
        op = nvme_log.get("current_self_test_operation", {})
        op_code = op.get("value", 0) if isinstance(op, dict) else op
        if op_code != 0:
            result["test_running"] = True
            result["test_remaining"] = nvme_log.get("current_self_test_completion_percent")
        table_entries = nvme_log.get("table", [])
        if table_entries:
            entry = table_entries[0]
            result["last_test_type"] = entry.get("self_test_code", {}).get("string", "")
            status_val = entry.get("self_test_result", {}).get("value", 0)
            result["last_test_ok"] = (status_val == 0)
            result["last_test_hours"] = entry.get("power_on_hours")
    else:
        st = d.get("ata_smart_data", {}).get("self_test", {})
        st_status = st.get("status", {})
        # value 15 (0x0f) means test in progress
        if st_status.get("value") == 15:
            result["test_running"] = True
            result["test_remaining"] = st_status.get("remaining_percent")
        log = d.get("ata_smart_self_test_log", {}).get("standard", {})
        entries = log.get("table", [])
        if entries:
            entry = entries[0]
            raw_type = entry.get("type", {}).get("string", "")
            if "extended" in raw_type.lower() or "long" in raw_type.lower():
                result["last_test_type"] = "Extended"
            else:
                result["last_test_type"] = "Short"
            st_str = entry.get("status", {}).get("string", "")
            result["last_test_ok"] = "without error" in st_str.lower()
            result["last_test_hours"] = entry.get("lifetime_hours")

    return result


def get_all_drives() -> list[dict]:
    scan = run("smartctl", "--scan")
    devs = [line.split()[0] for line in scan.splitlines() if line.split()]
    return [parse_drive(dev) for dev in devs]


def _base_dev(dev: str) -> str:
    """Strip partition suffix: nvme0n1p1 → nvme0n1, sda1 → sda."""
    m = re.match(r'(nvme\d+n\d+)p\d+$', dev)
    if m:
        return m.group(1)
    m = re.match(r'(sd[a-z]+)\d+$', dev)
    if m:
        return m.group(1)
    return dev


def build_disk_id_map() -> dict[str, str]:
    """Return {disk_id: base_dev_name} by reading /dev/disk/by-id/ symlinks."""
    result: dict = {}
    by_id = "/dev/disk/by-id"
    try:
        for entry in os.listdir(by_id):
            try:
                target = os.readlink(os.path.join(by_id, entry))
                dev = _base_dev(os.path.basename(target))
                result[entry] = dev
                # also index without trailing -partN so bare EUI names resolve too
                clean = re.sub(r'-part\d+$', '', entry)
                result.setdefault(clean, dev)
            except OSError:
                pass
    except OSError:
        pass
    return result


def get_iostat(pool: str) -> dict:
    """Return current read/write bandwidth in bytes/sec for a pool (1s sample)."""
    out = run("zpool", "iostat", "-Hp", pool, "1", "2")
    lines = [l for l in out.splitlines() if l.strip() and l.split('\t')[0].strip() == pool]
    # first line = cumulative since boot, second = 1s interval
    parts = (lines[-1] if lines else "").split('\t')
    try:
        return {"read": int(parts[5]), "write": int(parts[6])}
    except (IndexError, ValueError):
        return {"read": 0, "write": 0}


def parse_zpool_status() -> dict[str, dict]:
    """Parse zpool status into per-pool dicts with scan/errors/vdev topology."""
    out = run("zpool", "status")
    info: dict = {}
    pool: str | None = None
    in_config = False

    for line in out.splitlines():
        s = line.strip()

        if s.startswith("pool:"):
            pool = s.split(":", 1)[1].strip()
            info[pool] = {"scan": "", "errors": "No known data errors", "vdevs": []}
            in_config = False
            continue
        if pool is None:
            continue
        if s.startswith("scan:"):
            info[pool]["scan"] = s.split(":", 1)[1].strip()
        elif s.startswith("errors:"):
            info[pool]["errors"] = s.split(":", 1)[1].strip()
            in_config = False
        elif s == "config:":
            in_config = True
        elif in_config and line.startswith("\t") and s and "NAME" not in s:
            # Measure indent by spaces after the leading tab
            rest = line[1:]
            indent = len(rest) - len(rest.lstrip())
            parts = s.split()
            name, state = parts[0], (parts[1] if len(parts) > 1 else "")

            def parse_errors(p: list) -> dict:
                try:
                    return {"read": int(p[2]), "write": int(p[3]), "cksum": int(p[4])}
                except (IndexError, ValueError):
                    return {"read": 0, "write": 0, "cksum": 0}

            if indent == 0:
                info[pool].update(parse_errors(parts))
            elif indent == 2:
                vdev_type = name.split("-")[0] if re.match(r'(mirror|raidz\d?|draid)', name) else "disk"
                if vdev_type == "disk":
                    disk = {"id": name, "state": state, **parse_errors(parts)}
                    info[pool]["vdevs"].append({"type": "disk", "name": name, "state": state, "disks": [disk]})
                else:
                    info[pool]["vdevs"].append({"type": vdev_type, "name": name, "state": state, "disks": []})
            elif indent == 4 and info[pool]["vdevs"]:
                info[pool]["vdevs"][-1]["disks"].append({"id": name, "state": state, **parse_errors(parts)})

    return info


def get_zfs_pools() -> list[dict]:
    out = run("zpool", "list", "-Hp", "-o", "name,health,size,alloc,free,frag")
    if not out.strip():
        return []
    status = parse_zpool_status()
    disk_id_map = build_disk_id_map()
    pools = []
    for line in out.splitlines():
        parts = line.split("\t")
        if len(parts) < 6:
            continue
        name, health, size_b, alloc_b, free_b, frag = parts[:6]
        try:
            used_pct = round(int(alloc_b) / int(size_b) * 100)
        except (ValueError, ZeroDivisionError):
            used_pct = 0
        pinfo = status.get(name, {})
        # Resolve each disk id to its /dev name
        vdevs = pinfo.get("vdevs", [])
        for vdev in vdevs:
            for disk in vdev["disks"]:
                disk["dev"] = disk_id_map.get(disk["id"]) or (
                    disk["id"] if re.match(r'^(sd[a-z]+|nvme\d+n?\d*|hd[a-z]+)$', disk["id"]) else ""
                )
        size_int = int(size_b) if size_b.isdigit() else 0
        free_int = int(free_b) if free_b.isdigit() else 0
        pools.append({
            "name": name,
            "health": health,
            "size": fmt_bytes(size_int),
            "size_bytes": size_int,
            "free": fmt_bytes(free_int),
            "used_pct": used_pct,
            "frag": frag.rstrip("%"),
            "scan": pinfo.get("scan", ""),
            "errors": pinfo.get("errors", "No known data errors"),
            "vdevs": vdevs,
        })
    return pools


def get_data() -> tuple[list, list]:
    return get_all_drives(), get_zfs_pools()


def health_badge(passed: bool | None) -> str:
    if passed is True:
        return '<span class="badge ok">✓ PASS</span>'
    if passed is False:
        return '<span class="badge fail">✗ FAIL</span>'
    return '<span class="badge warn">? N/A</span>'


def pool_badge(state: str) -> str:
    cls = "ok" if state == "ONLINE" else "fail"
    return f'<span class="badge {cls}">{html_mod.escape(state)}</span>'


def stat_row(label: str, value: object, cls: str = "") -> str:
    """Single stats table row. cls: 'bad', 'warn', or '' for normal."""
    vcls = f' class="v{cls}"' if cls else ""
    return f'<tr><td>{label}</td><td{vcls}>{value}</td></tr>'


def counter_row(label: str, val: int | None, bad_if_nonzero: bool = True) -> str:
    """Row for a counter where 0 is good. bad_if_nonzero=True → red; False → yellow."""
    if val is None:
        return ""
    if val > 0:
        cls = "bad" if bad_if_nonzero else "warn"
    else:
        cls = "ok"
    return f'<tr><td>{label}</td><td class="v{cls}">{val:,}</td></tr>'


def hours_row(hours: int) -> str:
    if not hours:
        return ""
    y, rem = divmod(hours, 8760)
    label = f"{y}y {rem // 24}d" if y else f"{rem // 24}d"
    return stat_row("On time", f"{hours:,}h ({label})")


def render_drive_card(d: dict, pool_devs: set | None = None) -> str:
    if d.get("error"):
        return (
            f'<div class="card err-card">'
            f'<div class="dev">{html_mod.escape(d["dev"])}</div>'
            f'<div class="model">Error reading SMART data</div>'
            f'</div>'
        )

    temp = d.get("temp")
    temp_cls = "bad" if temp and temp >= 55 else "warn" if temp and temp >= 45 else ""
    temp_str = f"{temp}°C" if temp is not None else "—"

    rows = [stat_row("Temp", temp_str, temp_cls)]

    if d["type"] == "NVMe":
        spare = d.get("spare")
        thresh = d.get("spare_threshold", 10)
        if spare is not None:
            spare_cls = "bad" if spare <= thresh else "warn" if spare < 30 else "ok"
            rows.append(stat_row("Spare", f"{spare}% (min {thresh}%)", spare_cls))

        pused = d.get("pused")
        if pused is not None:
            wear_cls = "bad" if pused >= 90 else "warn" if pused >= 50 else "ok"
            rows.append(stat_row("Wear used", f"{pused}%", wear_cls))

        rows.append(counter_row("Media errors", d.get("media_errors"), bad_if_nonzero=True))
        rows.append(counter_row("Err log", d.get("err_log_entries"), bad_if_nonzero=True))

        if d.get("tbw") is not None:
            rows.append(stat_row("TBW", f"{d['tbw']} TB"))

    elif d["type"] == "HDD":
        # Critical: any non-zero means sectors are dying or lost
        rows.append(counter_row("Reallocated", d.get("reallocated"), bad_if_nonzero=True))
        rows.append(counter_row("Pending", d.get("pending"), bad_if_nonzero=True))
        rows.append(counter_row("Uncorr.", d.get("uncorrectable"), bad_if_nonzero=True))
        # Important: mechanical stress indicators
        rows.append(counter_row("Spin retries", d.get("spin_retries"), bad_if_nonzero=False))
        rows.append(counter_row("CRC errors", d.get("crc_errors"), bad_if_nonzero=False))
        # Informational
        if d.get("load_cycles") is not None:
            rows.append(stat_row("Load cycles", f"{d['load_cycles']:,}"))
        if d.get("start_stops") is not None:
            rows.append(stat_row("Start/stops", f"{d['start_stops']:,}"))

    else:  # SATA SSD
        rows.append(counter_row("Reallocated", d.get("reallocated"), bad_if_nonzero=True))
        rows.append(counter_row("Pending", d.get("pending"), bad_if_nonzero=True))
        rows.append(counter_row("Uncorr.", d.get("uncorrectable"), bad_if_nonzero=True))

        if d.get("wear_val") is not None:
            rows.append(stat_row("Wear left", f"{d['wear_val']}%"))

    rows.append(hours_row(d.get("hours") or 0))
    rows_html = "".join(r for r in rows if r)

    dtype = d["type"].lower()
    card_cls = "card" + (" fail-card" if d.get("health") is False else "")
    in_pool = (pool_devs is None) or (d["dev"] in pool_devs)
    unpool_badge = '' if in_pool else '<span class="tbadge t-unpool">NO POOL</span>'

    dev_js = html_mod.escape(d["dev"])
    test_running = d.get("test_running", False)
    if test_running:
        rem = d.get("test_remaining")
        rem_str = f" {rem}% left" if rem is not None else ""
        status_html = f'<span class="test-status running">⏳ Test running…{html_mod.escape(rem_str)}</span>'
    elif d.get("last_test_type") is not None:
        ok = d.get("last_test_ok")
        ltype = html_mod.escape(d["last_test_type"])
        hrs = d.get("last_test_hours")
        hrs_str = f" @ {hrs:,}h" if hrs is not None else ""
        if ok is True:
            status_html = f'<span class="test-status ok">✓ {ltype} OK{html_mod.escape(hrs_str)}</span>'
        elif ok is False:
            status_html = f'<span class="test-status fail">✗ {ltype} FAIL{html_mod.escape(hrs_str)}</span>'
        else:
            status_html = f'<span class="test-status">{ltype}{html_mod.escape(hrs_str)}</span>'
    else:
        status_html = ''

    disabled = ' disabled' if test_running else ''
    test_html = (
        f'<div class="card-tests">'
        f'{status_html}'
        f'<div class="test-btns">'
        f'<button class="test-btn"{disabled} onclick="startTest(this,\'{dev_js}\',\'short\')">Short</button>'
        f'<button class="test-btn"{disabled} onclick="startTest(this,\'{dev_js}\',\'long\')">Long</button>'
        f'</div>'
        f'</div>'
    )

    return f"""<div class="{card_cls}">
  <div class="card-head">
    <span class="dev">{html_mod.escape(d["dev"])}</span>
    <span class="tbadge t-{dtype}">{d["type"]}</span>{unpool_badge}
  </div>
  {f'<div class="brand">{html_mod.escape(d["brand"])}</div>' if d.get("brand") else ""}
  <div class="model" title="{html_mod.escape(d["model"])}">{html_mod.escape(d["model"][:28])}</div>
  <div class="drsize">{d["size"]}</div>
  <div class="hrow">{health_badge(d.get("health"))}</div>
  <table class="stats">{rows_html}</table>
  {test_html}
</div>"""


def render_pool_detail(pool_name: str) -> str:
    name_esc = html_mod.escape(pool_name)
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{name_esc} I/O</title>
  <style>
    *{{box-sizing:border-box;margin:0;padding:0}}
    body{{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#c9d1d9;min-height:100vh}}
    header{{background:#161b22;border-bottom:1px solid #30363d;padding:14px 24px;display:flex;align-items:center;gap:14px}}
    .back{{color:#58a6ff;text-decoration:none;font-size:.85rem}}
    .back:hover{{text-decoration:underline}}
    .htitle{{font-size:1.1rem;font-weight:700;color:#e6edf3}}
    .subtitle{{font-size:.8rem;color:#6e7681}}
    section{{padding:24px}}
    .speeds{{display:flex;gap:32px;margin-bottom:20px}}
    .speed-box{{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px 24px;min-width:160px}}
    .speed-lbl{{font-size:.7rem;text-transform:uppercase;letter-spacing:.08em;color:#8b949e;margin-bottom:6px}}
    .speed-val{{font-size:1.8rem;font-weight:700;font-variant-numeric:tabular-nums}}
    .speed-read{{color:#3fb950}}
    .speed-write{{color:#f0883e}}
    .graph-wrap{{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px}}
    .graph-toolbar{{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}}
    .legend{{display:flex;gap:20px;font-size:.75rem}}
    .leg-dot{{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:5px}}
    .win-btns{{display:flex;gap:6px}}
    .win-btn{{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer}}
    .win-btn:hover{{border-color:#58a6ff;color:#58a6ff}}
    .win-btn.active{{border-color:#58a6ff;background:#1a2d4a;color:#58a6ff}}
    canvas{{width:100%;height:300px;display:block}}
    @media (max-width:640px) {{
      header{{padding:10px 16px}}
      section{{padding:14px 16px}}
      .speeds{{flex-wrap:wrap;gap:12px}}
      .speed-box{{flex:1 1 120px}}
      .graph-toolbar{{flex-direction:column;gap:8px;align-items:flex-start}}
      .win-btn{{min-height:36px;padding:6px 12px}}
    }}
  </style>
</head>
<body>
  <header>
    <a class="back" href="/">← back</a>
    <span class="htitle">{name_esc}</span>
    <span class="subtitle">real-time I/O</span>
  </header>
  <section>
    <div class="speeds">
      <div class="speed-box">
        <div class="speed-lbl">Read</div>
        <div class="speed-val speed-read" id="read-val">—</div>
      </div>
      <div class="speed-box">
        <div class="speed-lbl">Write</div>
        <div class="speed-val speed-write" id="write-val">—</div>
      </div>
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
    const pool = {json.dumps(pool_name)};
    const BUFFER = 1800; // 30 min max
    let rBuf = [], wBuf = [], tBuf = []; // ring buffers with timestamps
    let winSecs = 60;
    const dpr = devicePixelRatio || 1;

    const canvas = document.getElementById('graph');
    const ctx = canvas.getContext('2d');

    function resize() {{
      canvas.width  = canvas.offsetWidth  * dpr;
      canvas.height = canvas.offsetHeight * dpr;
      draw();
    }}

    function setWin(s) {{
      winSecs = s;
      document.querySelectorAll('.win-btn').forEach(b => b.classList.remove('active'));
      document.getElementById('btn-' + s).classList.add('active');
      draw();
    }}

    function fmt(b) {{
      const mb = b / 1048576;
      return mb >= 100 ? mb.toFixed(0) + ' MB/s' : mb.toFixed(2) + ' MB/s';
    }}

    function timeFmt(ts, secs) {{
      const d = new Date(ts);
      const hh = String(d.getHours()).padStart(2,'0');
      const mm = String(d.getMinutes()).padStart(2,'0');
      const ss = String(d.getSeconds()).padStart(2,'0');
      return secs <= 300 ? `${{hh}}:${{mm}}:${{ss}}` : `${{hh}}:${{mm}}`;
    }}

    function draw() {{
      const n = Math.min(rBuf.length, winSecs);
      const r = rBuf.slice(-n), w = wBuf.slice(-n), t = tBuf.slice(-n);
      if (!r.length) return;

      const W = canvas.width, H = canvas.height;
      const PAD_L = 52 * dpr;  // left for y-axis labels
      const PAD_B = 26 * dpr;  // bottom for x-axis labels
      const GW = W - PAD_L, GH = H - PAD_B;

      ctx.clearRect(0, 0, W, H);
      ctx.fillStyle = '#0d1117';
      ctx.fillRect(0, 0, W, H);

      const peak = Math.max(1048576, ...r, ...w);
      const fs = Math.round(10 * dpr);
      ctx.font = `${{fs}}px monospace`;

      // ── Y axis grid + labels ──────────────────────────────────────
      [0.25, 0.5, 0.75, 1.0].forEach(f => {{
        const y = PAD_B + GH - GH * f * 0.92;
        ctx.strokeStyle = '#21262d'; ctx.lineWidth = 1;
        ctx.beginPath(); ctx.moveTo(PAD_L, y); ctx.lineTo(W, y); ctx.stroke();
        const lbl = (peak * f / 1048576).toFixed(f < 0.3 ? 2 : 1);
        ctx.fillStyle = '#6e7681';
        ctx.textAlign = 'right';
        ctx.fillText(lbl, PAD_L - 4, y + fs * 0.35);
      }});

      // ── X axis time ticks ─────────────────────────────────────────
      if (t.length >= 2) {{
        const t0 = t[0], t1 = t[t.length - 1], span = t1 - t0 || 1;
        const tickSecs = winSecs <= 60 ? 10 : winSecs <= 300 ? 30 : winSecs <= 900 ? 120 : 300;
        const tickMs = tickSecs * 1000;
        const firstTick = Math.ceil(t0 / tickMs) * tickMs;
        ctx.textAlign = 'center';
        for (let ts = firstTick; ts <= t1; ts += tickMs) {{
          const x = PAD_L + GW * (ts - t0) / span;
          ctx.strokeStyle = '#30363d'; ctx.lineWidth = 1;
          ctx.beginPath(); ctx.moveTo(x, PAD_B); ctx.lineTo(x, PAD_B + GH); ctx.stroke();
          ctx.fillStyle = '#6e7681';
          const lbl = timeFmt(ts, winSecs);
          const lw = ctx.measureText(lbl).width;
          const lx = Math.max(PAD_L + lw/2, Math.min(x, W - lw/2));
          ctx.fillText(lbl, lx, H - 5);
        }}
      }}

      // ── Data lines ────────────────────────────────────────────────
      function line(data, color) {{
        if (data.length < 2) return;
        ctx.strokeStyle = color; ctx.lineWidth = 2 * dpr;
        ctx.beginPath();
        data.forEach((v, i) => {{
          const x = PAD_L + GW * i / (data.length - 1);
          const y = PAD_B + GH - (v / peak) * GH * 0.92;
          i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
        }});
        ctx.stroke();
      }}

      line(r, '#3fb950');
      line(w, '#f0883e');
    }}

    async function poll() {{
      try {{
        const d = await fetch('/api/iostat/' + pool).then(r => r.json());
        rBuf.push(d.read);  wBuf.push(d.write);  tBuf.push(Date.now());
        if (rBuf.length > BUFFER) {{ rBuf.shift(); wBuf.shift(); tBuf.shift(); }}
        document.getElementById('read-val').textContent  = fmt(d.read);
        document.getElementById('write-val').textContent = fmt(d.write);
        draw();
      }} catch(e) {{}}
    }}

    window.addEventListener('resize', resize);
    resize();
    poll();
    setInterval(poll, 1000);
  </script>
</body>
</html>"""


def render_page(drives: list, pools: list) -> str:
    hostname = socket.gethostname()
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    fail_count = sum(1 for d in drives if d.get("health") is False)

    # Build a lookup from dev name → drive info for cross-referencing
    drive_by_dev = {d["dev"]: d for d in drives if not d.get("error")}

    # Collect every dev name used by any pool vdev.
    # ZFS resolves NVMe as nvme0n1; smartctl reports nvme0 — normalise both.
    pool_devs: set[str] = set()
    for p in pools:
        for vdev in p.get("vdevs", []):
            for disk in vdev["disks"]:
                dev = disk.get("dev", "")
                if dev:
                    pool_devs.add(dev)
                    pool_devs.add(re.sub(r'n\d+$', '', dev))   # nvme0n1 → nvme0

    cards_html = "\n".join(render_drive_card(d, pool_devs) for d in drives)

    pool_cards = []
    for p in pools:
        u = p["used_pct"]
        bar_col = "#e74c3c" if u > 90 else "#f39c12" if u > 75 else "#2ecc71"
        bar = f'<div class="bar"><div class="bfill" style="width:{u}%;background:{bar_col}"></div><span class="blbl">{u}%</span></div>'
        has_err = p["errors"] != "No known data errors"

        # Vdev structure HTML
        vdev_rows = []
        for vdev in p.get("vdevs", []):
            vtype = vdev["type"].upper()
            disk_rows = []
            for disk in vdev["disks"]:
                dev = disk.get("dev", "")
                dstate = disk["state"]
                dot_cls = "dok" if dstate == "ONLINE" else "dfail"
                # Look up drive info
                dr = drive_by_dev.get(dev, {})
                brand = html_mod.escape(dr.get("brand", ""))
                model = html_mod.escape(dr.get("model", "")[:30])
                dev_label = html_mod.escape(dev or disk["id"][:20])
                d_r, d_w, d_ck = disk.get("read", 0), disk.get("write", 0), disk.get("cksum", 0)
                any_derr = d_r or d_w or d_ck
                derr_cls = "disk-errs bad" if any_derr else "disk-errs"
                derr = f'<span class="{derr_cls}">R:{d_r} W:{d_w} CK:{d_ck}</span>'
                disk_rows.append(
                    f'<div class="disk-row">'
                    f'<span class="ddot {dot_cls}">●</span>'
                    f'<span class="ddev">{dev_label}</span>'
                    f'{f"<span class=\"dbrand\">{brand}</span>" if brand else ""}'
                    f'<span class="dmodel">{model}</span>'
                    f'{derr}'
                    f'</div>'
                )
            disks_html = "".join(disk_rows)
            if vdev["type"] == "disk":
                vdev_rows.append(
                    f'<div class="vdev-single">'
                    f'<span class="vtype">SINGLE</span>'
                    f'<div class="vdev-disks">{disks_html}</div>'
                    f'</div>'
                )
            else:
                vdev_rows.append(
                    f'<div class="vdev">'
                    f'<span class="vtype">{html_mod.escape(vtype)}</span>'
                    f'<div class="vdev-disks">{disks_html}</div>'
                    f'</div>'
                )

        vdevs_html = "".join(vdev_rows)

        def ec(val: int, label: str) -> str:
            cls = "ec-bad" if val > 0 else "ec-ok"
            return f'<span class="ec {cls}"><span class="ec-lbl">{label}</span>{val}</span>'

        errs_html = ec(p.get("read", 0), "R") + ec(p.get("write", 0), "W") + ec(p.get("cksum", 0), "CK")
        border_col = "#da3633" if p["health"] != "ONLINE" else ("#f39c12" if (p.get("read",0) or p.get("write",0) or p.get("cksum",0)) else "#238636")
        scrub_text = html_mod.escape(p["scan"] or "no scrub recorded")

        pool_cards.append(f'''<div class="pool-card" style="border-left-color:{border_col}">
  <div class="pc-head">
    <a class="pname" href="/pool/{html_mod.escape(p['name'])}">{html_mod.escape(p["name"])}</a>
    <span class="pc-right">
      <span class="psize">{p["size"]}</span>
      <span class="pdot">·</span>
      <span class="pfree">{p["free"]} free</span>
      <span class="pdot">·</span>
      <span class="pfrag">frag {html_mod.escape(str(p["frag"]))}%</span>
      <span class="pdot">·</span>
      <div class="pc-errs">{errs_html}</div>
      {pool_badge(p["health"])}
    </span>
  </div>
  <div class="pc-bar-wrap">
    <div class="pc-bar"><div class="pc-bfill" style="width:{u}%;background:{bar_col}"></div></div>
    <span class="pc-bpct">{u}%</span>
  </div>
  <div class="pc-vdevs">{vdevs_html}</div>
  <div class="pc-footer">
    <span class="pc-scrub {"pc-scrub-err" if has_err else ""}">{scrub_text}</span>
    <button class="scrub-btn" onclick="startScrub(this,'{html_mod.escape(p['name'])}')">▶ Scrub</button>
  </div>
</div>''')

    pool_html = '<div class="pool-list">' + "".join(pool_cards) + '</div>' if pool_cards else "<p>No ZFS pools found.</p>"

    banner = (
        f'<div class="fbanner">⚠ {fail_count} drive(s) reporting SMART failure</div>'
        if fail_count > 0 else ""
    )

    pool_names_js = json.dumps([p["name"] for p in pools])
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{html_mod.escape(hostname)} drives</title>
  <style>
    *{{box-sizing:border-box;margin:0;padding:0}}
    body{{font-family:'Segoe UI',system-ui,sans-serif;background:#0d1117;color:#c9d1d9;min-height:100vh}}
    header{{background:#161b22;border-bottom:1px solid #30363d;padding:14px 24px;display:flex;align-items:center;gap:12px}}
    .hname{{font-size:1.2rem;font-weight:700;color:#58a6ff}}
    .htitle{{color:#8b949e}}
    .hts{{font-size:.82rem;color:#6e7681;margin-left:auto}}
    .fbanner{{background:#3d1515;border-bottom:1px solid #da3633;color:#ff7b7b;padding:8px 24px;font-size:.88rem}}
    .tabbar{{background:#161b22;border-bottom:1px solid #30363d;display:flex;padding:0 24px;gap:4px}}
    .tab{{padding:10px 16px;font-size:.82rem;font-weight:600;border:none;background:none;color:#8b949e;cursor:pointer;border-bottom:2px solid transparent;transition:color .15s}}
    .tab.active{{color:#e6edf3;border-bottom-color:#58a6ff}}
    .tab:hover:not(.active){{color:#c9d1d9}}
    .tabpanel{{padding:20px 24px}}
    .tabpanel.hidden{{display:none}}
    section{{padding:20px 24px}}
    h2{{font-size:.72rem;text-transform:uppercase;letter-spacing:.1em;color:#6e7681;margin-bottom:14px}}
    .cards{{display:flex;flex-wrap:wrap;gap:12px}}
    .card{{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:14px;width:190px}}
    .card.fail-card{{border-color:#da3633;background:#1a1015}}
    .card.err-card{{border-color:#9e6a03;opacity:.7}}
    .card-head{{display:flex;justify-content:space-between;align-items:center;margin-bottom:3px}}
    .dev{{font-size:.95rem;font-weight:700;color:#e6edf3}}
    .brand{{font-size:.68rem;font-weight:600;color:#58a6ff;margin-bottom:1px;text-transform:uppercase;letter-spacing:.04em}}
    .model{{font-size:.73rem;color:#8b949e;margin-bottom:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}}
    .drsize{{font-size:.7rem;color:#6e7681;margin-bottom:8px}}
    .hrow{{margin-bottom:8px}}
    .badge{{display:inline-block;font-size:.7rem;font-weight:700;padding:2px 8px;border-radius:10px}}
    .badge.ok{{background:#1a3a1a;color:#3fb950;border:1px solid #238636}}
    .badge.fail{{background:#3a1a1a;color:#f85149;border:1px solid #da3633}}
    .badge.warn{{background:#3a2e1a;color:#d29922;border:1px solid #9e6a03}}
    .tbadge{{font-size:.62rem;font-weight:800;padding:2px 5px;border-radius:4px}}
    .t-nvme{{background:#1a2d4a;color:#58a6ff}}
    .t-ssd{{background:#1a3a2a;color:#3fb950}}
    .t-hdd{{background:#2e2a1a;color:#d29922}}
    .t-unpool{{background:#2e1f3a;color:#bc8cff;border:1px solid #6e40c9}}
    .stats{{width:100%;border-collapse:collapse;font-size:.73rem}}
    .stats td{{padding:2px 0}}
    .stats td:first-child{{color:#8b949e;width:76px}}
    .stats td:last-child{{color:#c9d1d9;font-variant-numeric:tabular-nums}}
    .vok{{color:#3fb950!important}}
    .vwarn{{color:#d29922!important}}
    .vbad{{color:#f85149!important;font-weight:700}}
    .pool-list{{display:grid;grid-template-columns:repeat(auto-fill,minmax(460px,1fr));gap:14px}}
    .pool-card{{background:#161b22;border:1px solid #30363d;border-left:3px solid #238636;border-radius:8px;padding:16px}}
    .pc-head{{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}}
    .pname{{font-size:1.05rem;font-weight:700;color:#e6edf3;text-decoration:none}}
    .pname:hover{{color:#58a6ff;text-decoration:underline}}
    .pc-right{{display:flex;align-items:center;gap:8px;flex-wrap:wrap}}
    .psize{{font-size:.78rem;color:#8b949e}}
    .pfree{{font-size:.78rem;color:#3fb950}}
    .pfrag{{font-size:.78rem;color:#6e7681}}
    .pdot{{color:#30363d;font-size:.8rem}}
    .pc-bar-wrap{{display:flex;align-items:center;gap:8px;margin-bottom:12px}}
    .pc-bar{{flex:1;background:#21262d;border-radius:6px;height:10px;overflow:hidden}}
    .pc-bfill{{height:100%;border-radius:6px;transition:width .3s}}
    .pc-bpct{{font-size:.72rem;font-weight:700;color:#c9d1d9;min-width:30px;text-align:right;font-variant-numeric:tabular-nums}}
    .pc-errs{{display:flex;gap:4px}}
    .ec{{display:inline-flex;align-items:center;gap:3px;font-size:.68rem;font-variant-numeric:tabular-nums;padding:1px 6px;border-radius:10px}}
    .ec-lbl{{font-weight:700;opacity:.7}}
    .ec-ok{{background:#1a2a1a;color:#8b949e;border:1px solid #21262d}}
    .ec-bad{{background:#3a1a1a;color:#f85149;border:1px solid #da3633;font-weight:700}}
    .pc-vdevs{{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:12px}}
    .vdev{{background:#0d1117;border:1px solid #21262d;border-radius:6px;padding:10px 12px;flex:1;min-width:200px}}
    .vdev-single{{background:#0d1117;border:1px solid #21262d;border-radius:6px;padding:10px 12px;flex:1}}
    .vtype{{display:inline-block;font-size:.6rem;font-weight:800;text-transform:uppercase;letter-spacing:.1em;color:#58a6ff;background:#1a2d4a;padding:1px 6px;border-radius:4px;margin-bottom:8px}}
    .vdev-disks{{display:flex;flex-direction:column;gap:5px}}
    .disk-row{{display:flex;align-items:center;gap:7px;font-size:.75rem}}
    .ddot{{font-size:.55rem;flex-shrink:0}}
    .dok{{color:#3fb950}}
    .dfail{{color:#f85149}}
    .ddev{{font-weight:700;color:#e6edf3;min-width:62px;flex-shrink:0}}
    .dbrand{{color:#58a6ff;font-size:.67rem;font-weight:600;min-width:44px;flex-shrink:0}}
    .dmodel{{color:#8b949e;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}}
    .disk-errs{{font-size:.67rem;color:#6e7681;font-variant-numeric:tabular-nums;flex-shrink:0;white-space:nowrap}}
    .disk-errs.bad{{color:#f85149;font-weight:700}}
    .pc-footer{{border-top:1px solid #21262d;padding-top:10px;display:flex;align-items:center;justify-content:space-between;gap:10px}}
    .pc-scrub{{font-size:.73rem;color:#6e7681;flex:1}}
    .pc-scrub-err{{color:#f85149}}
    .scrub-btn{{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #238636;background:#1a3a1a;color:#3fb950;cursor:pointer;white-space:nowrap;flex-shrink:0}}
    .scrub-btn:hover{{background:#238636;color:#fff}}
    .scrub-btn:disabled{{opacity:.5;cursor:default}}
    .bar{{background:#21262d;border-radius:4px;height:16px;min-width:100px;position:relative}}
    .bfill{{height:100%;border-radius:4px}}
    .blbl{{position:absolute;right:5px;top:0;font-size:.68rem;line-height:16px;color:#e6edf3;font-weight:700}}
    .io-toolbar{{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px}}
    .io-toolbar-legend{{display:flex;gap:16px;font-size:.75rem}}
    .io-leg-dot{{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:4px}}
    .win-btns{{display:flex;gap:6px}}
    .win-btn{{font-size:.72rem;font-weight:700;padding:3px 10px;border-radius:6px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer}}
    .win-btn:hover{{border-color:#58a6ff;color:#58a6ff}}
    .win-btn.active{{border-color:#58a6ff;background:#1a2d4a;color:#58a6ff}}
    .io-grid{{display:grid;grid-template-columns:repeat(auto-fill,minmax(420px,1fr));gap:14px}}
    .io-card{{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:14px}}
    .io-card-head{{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px}}
    .io-pname{{font-size:.95rem;font-weight:700;color:#e6edf3;text-decoration:none}}
    .io-pname:hover{{color:#58a6ff}}
    .io-speeds{{display:flex;gap:20px;margin-bottom:8px;font-size:.82rem;font-variant-numeric:tabular-nums}}
    .io-read{{color:#3fb950;font-weight:600}}
    .io-write{{color:#f0883e;font-weight:600}}
    .io-lbl{{color:#6e7681;font-size:.7rem;margin-right:4px}}
    canvas.io-canvas{{width:100%;height:140px;display:block;border-radius:4px;background:#0d1117}}
    .card-tests{{border-top:1px solid #21262d;margin-top:8px;padding-top:7px;display:flex;flex-direction:column;gap:5px}}
    .test-status{{font-size:.68rem;color:#6e7681}}
    .test-status.running{{color:#d29922}}
    .test-status.ok{{color:#3fb950}}
    .test-status.fail{{color:#f85149;font-weight:700}}
    .test-btns{{display:flex;gap:5px}}
    .test-btn{{font-size:.68rem;font-weight:700;padding:2px 8px;border-radius:5px;border:1px solid #30363d;background:#21262d;color:#8b949e;cursor:pointer;flex:1}}
    .test-btn:hover:not(:disabled){{border-color:#58a6ff;color:#58a6ff}}
    .test-btn:disabled{{opacity:.45;cursor:default}}
    @media (max-width:640px) {{
      header{{padding:10px 16px;gap:8px}}
      .hts{{display:none}}
      .tabbar{{padding:0 8px;overflow-x:auto;-webkit-overflow-scrolling:touch}}
      .tab{{padding:8px 10px;font-size:.76rem;white-space:nowrap}}
      .tabpanel{{padding:14px 16px}}
      .card{{width:100%;max-width:100%}}
      .pool-list{{grid-template-columns:1fr}}
      .io-grid{{grid-template-columns:1fr}}
      .pc-head{{flex-direction:column;align-items:flex-start;gap:4px}}
      .io-toolbar{{flex-direction:column;gap:8px;align-items:flex-start}}
      .io-card-head{{flex-direction:column;align-items:flex-start;gap:4px}}
      .scrub-btn,.win-btn{{min-height:36px;padding:6px 12px}}
      .test-btn{{min-height:36px;padding:6px 8px}}
    }}
  </style>
</head>
<body>
  <header>
    <span class="hname">{html_mod.escape(hostname)}</span>
    <span class="htitle">drive health</span>
    <span class="hts">refreshed {now}</span>
  </header>
  {banner}
  <div class="tabbar">
    <button class="tab active" onclick="showTab('drives',this)">Drives ({len(drives)})</button>
    <button class="tab"        onclick="showTab('pools',this)">Pools ({len(pools)})</button>
    <button class="tab"        onclick="showTab('io',this)">Active I/O</button>
  </div>

  <div id="tab-drives" class="tabpanel">
    <h2>{fmt_bytes(sum(d.get("cap_bytes", 0) for d in drives))} total raw capacity across {len(drives)} drives</h2>
    <br>
    <div class="cards">{cards_html}</div>
  </div>

  <div id="tab-pools" class="tabpanel hidden">
    <h2>{fmt_bytes(sum(p.get("size_bytes", 0) for p in pools))} total usable capacity across {len(pools)} pools</h2>
    <br>
    {pool_html}
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
    // ── Tab switching ────────────────────────────────────────────────
    function showTab(name, btn) {{
      document.querySelectorAll('.tabpanel').forEach(p => p.classList.add('hidden'));
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.getElementById('tab-' + name).classList.remove('hidden');
      btn.classList.add('active');
      if (name === 'io') startIO(); else stopIO();
    }}

    // ── Scrub ────────────────────────────────────────────────────────
    function startScrub(btn, pool) {{
      btn.disabled = true; btn.textContent = '⏳ Starting…';
      fetch('/scrub/' + pool, {{method:'POST'}})
        .then(r => r.ok
          ? (btn.textContent = '✓ Started', setTimeout(()=>location.reload(), 1500))
          : (btn.textContent = '✗ Error',   btn.disabled = false))
        .catch(() => (btn.textContent = '✗ Error', btn.disabled = false));
    }}

    // ── SMART self-test ──────────────────────────────────────────────
    function startTest(btn, dev, kind) {{
      const btns = btn.parentElement.querySelectorAll('.test-btn');
      btns.forEach(b => b.disabled = true);
      btn.textContent = '⏳ Starting…';
      fetch('/test/' + dev + '/' + kind, {{method:'POST'}})
        .then(r => r.ok
          ? (btn.textContent = '✓ Started', setTimeout(()=>location.reload(), 2000))
          : (btn.textContent = '✗ Error',   btns.forEach(b => b.disabled = false)))
        .catch(() => (btn.textContent = '✗ Error', btns.forEach(b => b.disabled = false)));
    }}

    // ── Active I/O ───────────────────────────────────────────────────
    const IO_POOLS = {pool_names_js};
    const IO_BUF = 1800;
    const dpr = devicePixelRatio || 1;
    let ioState = {{}};   // pool → {{r,w,t,canvas,ctx,rEl,wEl}}
    let ioWin = 60, ioTick = null;

    function fmtMB(b) {{
      const mb = b / 1048576;
      return mb >= 100 ? mb.toFixed(0) + ' MB/s' : mb.toFixed(2) + ' MB/s';
    }}

    function timeFmt(ts) {{
      const d = new Date(ts);
      const hh = String(d.getHours()).padStart(2,'0');
      const mm = String(d.getMinutes()).padStart(2,'0');
      const ss = String(d.getSeconds()).padStart(2,'0');
      return ioWin <= 300 ? `${{hh}}:${{mm}}:${{ss}}` : `${{hh}}:${{mm}}`;
    }}

    function initIO() {{
      const grid = document.getElementById('io-grid');
      if (grid.children.length) return;
      IO_POOLS.forEach(pool => {{
        const card = document.createElement('div');
        card.className = 'io-card';
        card.innerHTML = `
          <div class="io-card-head">
            <a class="io-pname" href="/pool/${{pool}}">${{pool}}</a>
            <div class="io-speeds">
              <span><span class="io-lbl">R</span><span class="io-read" id="io-r-${{pool}}">—</span></span>
              <span><span class="io-lbl">W</span><span class="io-write" id="io-w-${{pool}}">—</span></span>
            </div>
          </div>
          <canvas class="io-canvas" id="io-c-${{pool}}"></canvas>`;
        grid.appendChild(card);
        const canvas = document.getElementById('io-c-' + pool);
        canvas.width  = canvas.offsetWidth  * dpr;
        canvas.height = canvas.offsetHeight * dpr;
        ioState[pool] = {{
          r: [], w: [], t: [],
          canvas, ctx: canvas.getContext('2d'),
          rEl: document.getElementById('io-r-' + pool),
          wEl: document.getElementById('io-w-' + pool),
        }};
      }});
    }}

    function startIO() {{
      initIO();
      if (!ioTick) {{ pollIO(); ioTick = setInterval(pollIO, 1000); }}
    }}
    function stopIO() {{
      if (ioTick) {{ clearInterval(ioTick); ioTick = null; }}
    }}

    function setIOWin(s) {{
      ioWin = s;
      document.querySelectorAll('.win-btn').forEach(b => b.classList.remove('active'));
      document.getElementById('io-btn-' + s).classList.add('active');
      IO_POOLS.forEach(drawIO);
    }}

    async function pollIO() {{
      await Promise.all(IO_POOLS.map(async pool => {{
        try {{
          const d = await fetch('/api/iostat/' + pool).then(r => r.json());
          const s = ioState[pool];
          if (!s) return;
          s.r.push(d.read); s.w.push(d.write); s.t.push(Date.now());
          if (s.r.length > IO_BUF) {{ s.r.shift(); s.w.shift(); s.t.shift(); }}
          s.rEl.textContent = fmtMB(d.read);
          s.wEl.textContent = fmtMB(d.write);
          drawIO(pool);
        }} catch(e) {{}}
      }}));
    }}

    function drawIO(pool) {{
      const s = ioState[pool];
      if (!s) return;
      const n   = Math.min(s.r.length, ioWin);
      const r   = s.r.slice(-n), w = s.w.slice(-n), t = s.t.slice(-n);
      const {{canvas, ctx}} = s;
      const W = canvas.width, H = canvas.height;
      const PAD_L = 44 * dpr, PAD_B = 20 * dpr;
      const GW = W - PAD_L, GH = H - PAD_B;

      ctx.clearRect(0, 0, W, H);
      ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, W, H);

      const peak = Math.max(1048576, ...r, ...w);
      const fs = Math.round(9 * dpr);
      ctx.font = `${{fs}}px monospace`;

      // Y grid + labels
      [0.5, 1.0].forEach(f => {{
        const y = PAD_B + GH - GH * f * 0.9;
        ctx.strokeStyle = '#21262d'; ctx.lineWidth = 1;
        ctx.beginPath(); ctx.moveTo(PAD_L, y); ctx.lineTo(W, y); ctx.stroke();
        ctx.fillStyle = '#6e7681'; ctx.textAlign = 'right';
        ctx.fillText((peak * f / 1048576).toFixed(1), PAD_L - 3, y + fs * 0.35);
      }});

      // X time ticks
      if (t.length >= 2) {{
        const t0 = t[0], t1 = t[t.length-1], span = t1 - t0 || 1;
        const tickSecs = ioWin <= 60 ? 10 : ioWin <= 300 ? 30 : ioWin <= 900 ? 120 : 300;
        const firstTick = Math.ceil(t0 / (tickSecs*1000)) * tickSecs * 1000;
        ctx.textAlign = 'center';
        for (let ts = firstTick; ts <= t1; ts += tickSecs*1000) {{
          const x = PAD_L + GW * (ts - t0) / span;
          ctx.strokeStyle = '#30363d'; ctx.lineWidth = 1;
          ctx.beginPath(); ctx.moveTo(x, PAD_B); ctx.lineTo(x, PAD_B + GH); ctx.stroke();
          ctx.fillStyle = '#6e7681';
          const lbl = timeFmt(ts), lw = ctx.measureText(lbl).width;
          ctx.fillText(lbl, Math.max(PAD_L + lw/2, Math.min(x, W - lw/2)), H - 3);
        }}
      }}

      // Lines
      function line(data, color) {{
        if (data.length < 2) return;
        ctx.strokeStyle = color; ctx.lineWidth = 1.5 * dpr; ctx.beginPath();
        data.forEach((v, i) => {{
          const x = PAD_L + GW * i / (data.length - 1);
          const y = PAD_B + GH - (v / peak) * GH * 0.9;
          i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
        }});
        ctx.stroke();
      }}
      line(r, '#3fb950');
      line(w, '#f0883e');
    }}

    setTimeout(()=>location.reload(), 86400000);
  </script>
</body>
</html>"""


class Handler(http.server.BaseHTTPRequestHandler):
    def _send(self, code: int, body: bytes, ct: str = "text/html; charset=utf-8") -> None:
        self.send_response(code)
        self.send_header("Content-Type", ct)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path in ("/", "/index.html"):
            drives, pools = get_data()
            self._send(200, render_page(drives, pools).encode())
            return

        m = re.match(r'^/pool/([a-zA-Z0-9_\-]+)$', self.path)
        if m:
            self._send(200, render_pool_detail(m.group(1)).encode())
            return

        m = re.match(r'^/api/iostat/([a-zA-Z0-9_\-]+)$', self.path)
        if m:
            data = get_iostat(m.group(1))
            self._send(200, json.dumps(data).encode(), "application/json")
            return

        self._send(404, b"not found")

    def do_POST(self) -> None:
        m = re.match(r'^/scrub/([a-zA-Z0-9_\-]+)$', self.path)
        if m:
            run("zpool", "scrub", m.group(1))
            self._send(200, b"ok", "text/plain")
            return

        m = re.match(r'^/test/([a-zA-Z0-9_\-]+)/(short|long)$', self.path)
        if m:
            dev, kind = m.group(1), m.group(2)
            run("smartctl", "-t", kind, f"/dev/{dev}")
            self._send(200, b"ok", "text/plain")
            return

        self._send(400, b"bad request")

    def log_message(self, fmt: str, *args: object) -> None:
        pass


class ThreadingServer(socketserver.ThreadingMixIn, socketserver.TCPServer):
    allow_reuse_address = True
    daemon_threads = True


if __name__ == "__main__":
    with ThreadingServer(("", PORT), Handler) as srv:
        print(f"Drive monitor on http://0.0.0.0:{PORT}", flush=True)
        srv.serve_forever()
