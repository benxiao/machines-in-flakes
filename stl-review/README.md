# stl-review

3D viewer + annotation tool for the print-design loop: view a part, diff it against an earlier
version, and leave notes pinned to exact model coordinates for Claude to act on.

Runs on athena at `http://athena:10095` (declared in `flake.nix` via `makeGoService`).

## Projects

Any `~/<project>/stl/` directory is a project. Files named `<part>_v<N>.stl` group into a part with
versions; `scan/` and `ref/` subdirectories hold reference meshes (e.g. a 3D scan) to compare against.
The page polls every 3 s: when a newer version of the part on screen appears, it switches to it and
diffs against the previous one automatically.

## Notes API

Every note stores the point (model mm, same frame as the CAD script), surface normal, optional second
point (measurements), camera pose, and a PNG screenshot with the point and text burned in.

```bash
# open notes for a project
curl -s 'localhost:10095/api/annotations?project=cam-adapter&status=open'
# screenshot (shot_file in the JSON): /var/lib/stl-review/shots/<id>.png

# after addressing a note in a new version
curl -s -X PATCH localhost:10095/api/annotations/12 \
  -d '{"status":"resolved","resolved_in":"stl/ascent_plus_lens_hood_v6.stl","reply":"Lip +0.5 mm (2.0 → 2.5)"}'
```

Filters: `project`, `part`, `file`, `status` (`open` | `resolved` | `wontfix`).

`kind` is `pin`, `measure` (`point` → `point2`), `paint` (the Brush tool, current), or `circle` / `rect`
(older area notes, still displayed). Area notes carry `region`:
`centroid`, `size` and `bbox` (mm), `area_mm2`, average `normal`, up to 250 `samples` (surface points inside
the shape, from a ray grid), and `ndc` (the shape in the drawing camera's normalised coordinates, with
`camera` saved alongside). `point` is the sampled surface point nearest the centroid. The page re-projects
the shape from that camera, so the highlight lands on any version's surface.

A `paint` region has `brush.dabs`: an ordered list of `[x, y, z, ±r, nx, ny, nz]` (mm). A surface point p
with normal n is painted when the **last** dab with `|p − (x,y,z)| < r` and `n · (nx,ny,nz) > 0` has a
positive r (negative = eraser). The normal test keeps the far side of a thin wall unpainted. `samples`,
`centroid`, `bbox`, `area_mm2` are computed from that rule on the version the note was made on.

## Frontend

`static/` is embedded into the binary. `static/vendor.js` is three.js + OrbitControls + STLLoader +
three-mesh-bvh bundled into one file by `jsbundle/build.sh` (committed, so the Nix build needs no network).
