package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer, q *Queries) {
	s.AddTool(
		mcp.NewTool("list_drones",
			mcp.WithDescription("List all FPV drones with status and component summary"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			drones, err := q.ListDrones(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatDroneList(drones)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_drone",
			mcp.WithDescription("Get full detail for one drone including all components"),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Drone ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("id must be a positive integer"), nil
			}
			drone, err := q.GetDrone(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("drone #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatDroneDetail(drone)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_sessions",
			mcp.WithDescription("List recent flight/maintenance/crash sessions with drone and battery info"),
			mcp.WithNumber("limit", mcp.Description("Max sessions to return (default 20)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			limit := req.GetInt("limit", 20)
			sessions, err := q.ListSessions(ctx, limit)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatSessionList(sessions)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_session",
			mcp.WithDescription("Get full detail for one session including drones, batteries, and counts"),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Session ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("id must be a positive integer"), nil
			}
			detail, err := q.GetSession(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("session #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatSessionDetail(detail)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_batteries",
			mcp.WithDescription("List all battery packs with status, capacity, and assigned drone"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			bats, err := q.ListBatteries(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatBatteryList(bats)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_components",
			mcp.WithDescription("List inventory components (frames, FCs, ESCs, motors, VTX, GPS, RX) with stock/installed/available counts"),
			mcp.WithString("type",
				mcp.Description("Filter by component type. Omit for all types."),
				mcp.Enum("frame", "fc", "esc", "motor", "vtx", "gps", "rx"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			typeFilter := req.GetString("type", "")
			comps, err := q.ListComponents(ctx, typeFilter)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatComponentList(comps)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_places",
			mcp.WithDescription("List all flying locations with address and coordinates"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			places, err := q.ListPlaces(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatPlaceList(places)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_drone_log",
			mcp.WithDescription("Get the maintenance/build log entries for a drone"),
			mcp.WithNumber("drone_id", mcp.Required(), mcp.Description("Drone ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("drone_id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("drone_id must be a positive integer"), nil
			}
			_, err := q.GetDrone(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("drone #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			entries, err := q.ListDroneLogs(ctx, id)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatDroneLog(id, entries)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_brands",
			mcp.WithDescription("List all brands"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			brands, err := q.ListBrands(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatBrandList(brands)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("create_brand",
			mcp.WithDescription("Create a new brand"),
			mcp.WithString("name", mcp.Required(), mcp.Description("Brand name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := strings.TrimSpace(req.GetString("name", ""))
			if name == "" {
				return mcp.NewToolResultText("name is required"), nil
			}
			b, err := q.CreateBrand(ctx, name)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Created brand #%d: %s", b.ID, b.Name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("update_brand",
			mcp.WithDescription("Rename a brand"),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Brand ID")),
			mcp.WithString("name", mcp.Required(), mcp.Description("New brand name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("id must be a positive integer"), nil
			}
			name := strings.TrimSpace(req.GetString("name", ""))
			if name == "" {
				return mcp.NewToolResultText("name is required"), nil
			}
			b, err := q.UpdateBrand(ctx, id, name)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("brand #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Updated brand #%d: %s", b.ID, b.Name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("delete_brand",
			mcp.WithDescription("Delete a brand (only if no components reference it)"),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("Brand ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := req.GetInt("id", 0)
			if id <= 0 {
				return mcp.NewToolResultText("id must be a positive integer"), nil
			}
			err := q.DeleteBrand(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return mcp.NewToolResultText(fmt.Sprintf("brand #%d not found", id)), nil
			}
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(fmt.Sprintf("Deleted brand #%d", id)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_low_stock",
			mcp.WithDescription("List propellers and spare parts at or below their reorder threshold"),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			props, spares, err := q.ListLowStock(ctx)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResultText(formatLowStock(props, spares)), nil
		},
	)
}

// ---- Formatters ----

func formatBrandList(brands []BrandRow) string {
	if len(brands) == 0 {
		return "No brands found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d brand(s):\n", len(brands))
	for _, br := range brands {
		fmt.Fprintf(&b, "  #%d: %s\n", br.ID, br.Name)
	}
	return b.String()
}

func compName(brand, name string) string {
	switch {
	case brand != "" && name != "":
		return brand + " " + name
	case name != "":
		return name
	case brand != "":
		return brand
	default:
		return "—"
	}
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func formatDroneList(drones []DroneRow) string {
	if len(drones) == 0 {
		return "No drones found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d drone(s):\n", len(drones))
	for _, d := range drones {
		fmt.Fprintf(&b, "\n#%d: %s  [%s]", d.ID, d.Name, d.Status)
		if d.SizeLabel != "" {
			fmt.Fprintf(&b, "  %s\"", d.SizeLabel)
		}
		if d.CellLabel != "" {
			fmt.Fprintf(&b, "  %s", d.CellLabel)
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "  Frame: %s  FC: %s  ESC: %s  VTX: %s\n",
			compName(d.FrameBrand, d.FrameName),
			compName(d.FCBrand, d.FCName),
			compName(d.ESCBrand, d.ESCName),
			compName(d.VTXBrand, d.VTXName))
		fmt.Fprintf(&b, "  Motors: %s  GPS: %s  RX: %s\n",
			formatMotors(d.MotorBrand, d.MotorName, d.MotorCount),
			compName(d.GPSBrand, d.GPSName),
			compName(d.RXBrand, d.RXName))
		if d.BattNames != "" {
			fmt.Fprintf(&b, "  Batteries: %s\n", d.BattNames)
		}
		if d.BuildDate != nil && *d.BuildDate != "" {
			fmt.Fprintf(&b, "  Built: %s\n", *d.BuildDate)
		}
		if d.WeightG != nil {
			sub250 := ""
			if d.Sub250g {
				sub250 = "  sub250g"
			}
			fmt.Fprintf(&b, "  Weight: %dg%s\n", *d.WeightG, sub250)
		} else if d.Sub250g {
			fmt.Fprintf(&b, "  sub250g\n")
		}
	}
	return b.String()
}

func formatDroneDetail(d DroneRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Drone #%d: %s  [%s]\n\n", d.ID, d.Name, d.Status)
	if d.SizeLabel != "" {
		fmt.Fprintf(&b, "  Size:        %s\"\n", d.SizeLabel)
	}
	if d.CellLabel != "" {
		fmt.Fprintf(&b, "  Cell count:  %s\n", d.CellLabel)
	}
	fmt.Fprintf(&b, "  Frame:       %s\n", compName(d.FrameBrand, d.FrameName))
	fmt.Fprintf(&b, "  FC:          %s\n", compName(d.FCBrand, d.FCName))
	fmt.Fprintf(&b, "  ESC:         %s\n", compName(d.ESCBrand, d.ESCName))
	fmt.Fprintf(&b, "  VTX:         %s\n", compName(d.VTXBrand, d.VTXName))
	fmt.Fprintf(&b, "  Motors:      %s\n", formatMotors(d.MotorBrand, d.MotorName, d.MotorCount))
	fmt.Fprintf(&b, "  Batteries:   %s\n", dash(d.BattNames))
	fmt.Fprintf(&b, "  GPS:         %s\n", compName(d.GPSBrand, d.GPSName))
	fmt.Fprintf(&b, "  RX:          %s\n", compName(d.RXBrand, d.RXName))
	buildDate := "—"
	if d.BuildDate != nil && *d.BuildDate != "" {
		buildDate = *d.BuildDate
	}
	fmt.Fprintf(&b, "  Build date:  %s\n", buildDate)
	if d.WeightG != nil {
		fmt.Fprintf(&b, "  Weight:      %dg\n", *d.WeightG)
	}
	if d.Sub250g {
		fmt.Fprintf(&b, "  Sub 250g:    yes\n")
	}
	if d.Notes != "" {
		fmt.Fprintf(&b, "  Notes:       %s\n", d.Notes)
	}
	return b.String()
}

func formatMotors(brand, name string, count int) string {
	n := compName(brand, name)
	if n == "—" {
		return "—"
	}
	return fmt.Sprintf("%s x%d", n, count)
}

func formatSessionList(sessions []SessionRow) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d session(s):\n", len(sessions))
	for _, s := range sessions {
		title := ""
		if s.Title != "" {
			title = ": " + s.Title
		}
		dur := "—"
		if s.DurationMin > 0 {
			dur = fmt.Sprintf("%d min", s.DurationMin)
		}
		fmt.Fprintf(&b, "\n#%d%s  [%s]  %s  %s  %s\n",
			s.ID, title, s.Type, s.SessionDate, dur, dash(s.Location))
		if s.DroneNames != "" {
			fmt.Fprintf(&b, "  Drones: %s\n", s.DroneNames)
		}
		if s.BatteryList != "" {
			fmt.Fprintf(&b, "  Batteries: %s\n", s.BatteryList)
		}
		if s.Notes != "" {
			fmt.Fprintf(&b, "  Notes: %s\n", s.Notes)
		}
	}
	return b.String()
}

func formatSessionDetail(d SessionDetail) string {
	s := d.Session
	var b strings.Builder
	title := fmt.Sprintf("Session #%d", s.ID)
	if s.Title != "" {
		title = fmt.Sprintf("Session #%d: %s", s.ID, s.Title)
	}
	fmt.Fprintf(&b, "%s  [%s]\n", title, s.Type)
	fmt.Fprintf(&b, "  Date:      %s\n", s.SessionDate)
	dur := "—"
	if s.DurationMin > 0 {
		dur = fmt.Sprintf("%d min", s.DurationMin)
	}
	fmt.Fprintf(&b, "  Duration:  %s\n", dur)
	fmt.Fprintf(&b, "  Location:  %s\n", dash(s.Location))
	if len(d.Drones) > 0 {
		names := make([]string, len(d.Drones))
		for i, dr := range d.Drones {
			names[i] = fmt.Sprintf("%s (#%d)", dr.Name, dr.ID)
		}
		fmt.Fprintf(&b, "  Drones:    %s\n", strings.Join(names, ", "))
	} else {
		fmt.Fprintf(&b, "  Drones:    —\n")
	}
	if len(d.Batteries) > 0 {
		parts := make([]string, len(d.Batteries))
		for i, bt := range d.Batteries {
			parts[i] = fmt.Sprintf("%s %s %dmAh x%d",
				compName(bt.Brand, bt.Name), bt.CellLabel, bt.CapacityMAh, bt.Count)
		}
		fmt.Fprintf(&b, "  Batteries: %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Fprintf(&b, "  Batteries: —\n")
	}
	if s.Notes != "" {
		fmt.Fprintf(&b, "  Notes:     %s\n", s.Notes)
	}
	return b.String()
}

func formatBatteryList(bats []BatteryRow) string {
	if len(bats) == 0 {
		return "No batteries found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d battery pack(s):\n", len(bats))
	for _, bt := range bats {
		weight := ""
		if bt.WeightG != nil {
			weight = fmt.Sprintf("  %dg", *bt.WeightG)
		}
		fmt.Fprintf(&b, "\n#%d: %s  %s %dmAh%s  qty:%d  [%s]\n",
			bt.ID, compName(bt.Brand, bt.Name), bt.CellLabel, bt.CapacityMAh, weight, bt.Count, bt.Status)
		if bt.AssignedTo != "" {
			fmt.Fprintf(&b, "  Assigned to: %s\n", bt.AssignedTo)
		}
		if bt.Notes != "" {
			fmt.Fprintf(&b, "  Notes: %s\n", bt.Notes)
		}
	}
	return b.String()
}

func formatComponentList(comps []ComponentRow) string {
	if len(comps) == 0 {
		return "No components found."
	}
	var b strings.Builder
	currentType := ""
	for _, c := range comps {
		if c.Type != currentType {
			if currentType != "" {
				fmt.Fprintln(&b)
			}
			currentType = c.Type
			fmt.Fprintf(&b, "%s:\n", strings.ToUpper(currentType))
		}
		label := compName(c.Brand, c.Name)
		specs := ""
		if c.Specs != "" {
			specs = "  [" + c.Specs + "]"
		}
		installed := ""
		if c.InstalledOn != "" {
			installed = "  → " + c.InstalledOn
		}
		fmt.Fprintf(&b, "  #%d  %s%s  stock:%d  installed:%d  avail:%d%s\n",
			c.ID, label, specs, c.Total, c.Installed, c.Available, installed)
	}
	return b.String()
}

func formatPlaceList(places []PlaceRow) string {
	if len(places) == 0 {
		return "No places found."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d place(s):\n", len(places))
	for _, p := range places {
		fmt.Fprintf(&b, "\n#%d: %s  [%s]\n", p.ID, p.Name, p.PlaceType)
		if p.Address != "" {
			fmt.Fprintf(&b, "  Address:  %s\n", p.Address)
		}
		if p.Lat != nil && p.Lng != nil {
			fmt.Fprintf(&b, "  Location: %.4f, %.4f\n", *p.Lat, *p.Lng)
		}
		if p.Notes != "" {
			fmt.Fprintf(&b, "  Notes:    %s\n", p.Notes)
		}
	}
	return b.String()
}

func formatDroneLog(droneID int, entries []DroneLogEntry) string {
	if len(entries) == 0 {
		return fmt.Sprintf("No log entries for drone #%d.", droneID)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d log entry(ies) for drone #%d:\n", len(entries), droneID)
	for _, e := range entries {
		fmt.Fprintf(&b, "\n[%s] #%d\n%s\n", e.LoggedAt, e.ID, e.Body)
	}
	return b.String()
}

func formatLowStock(props []LowPropRow, spares []LowSpareRow) string {
	total := len(props) + len(spares)
	if total == 0 {
		return "All propellers and spare parts are above reorder thresholds."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "LOW STOCK — %d item(s) need reordering:\n", total)
	if len(props) > 0 {
		fmt.Fprintf(&b, "\nPropellers:\n")
		for _, p := range props {
			size := ""
			if p.SizeLabel != "" {
				size = fmt.Sprintf(" %s\"", p.SizeLabel)
			}
			drone := ""
			if p.DroneNames != "" {
				drone = fmt.Sprintf(" (for: %s)", p.DroneNames)
			}
			fmt.Fprintf(&b, "  %s%s %dx blades%s  qty:%d  threshold:%d\n",
				compName(p.Brand, p.Name), size, p.BladeCount, drone, p.Quantity, p.ReorderThreshold)
		}
	}
	if len(spares) > 0 {
		fmt.Fprintf(&b, "\nSpare parts:\n")
		for _, s := range spares {
			fmt.Fprintf(&b, "  [%s] %s  qty:%d  threshold:%d\n",
				s.Category, s.Name, s.Quantity, s.ReorderThreshold)
		}
	}
	return b.String()
}
