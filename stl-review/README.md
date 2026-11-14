# stl-review

3D viewer + annotation tool for the print-design loop: view a part, diff it against an earlier
version, and leave notes pinned to exact model coordinates for Claude to act on.

Runs on athena at `http://athena:10095` (declared in `flake.nix` via `makeGoService`).

## Projects

Any `~/<project>/stl/` directory is a project, even an empty one. Files named `<part>_v<N>.stl` group into a part with
versions; `scan/` and `ref/` subdirectories hold reference meshes (e.g. a 3D scan) to compare against.
The page polls every 3 s: when a newer version of the part on screen appears, it switches to it and
diffs against the previous one automatically.

**New project…** at the bottom of the project menu creates an empty `~/<name>/stl/`. The page shows it
as an empty bed with an upload button, and switches to its first part as soon as one is uploaded or a
release script saves one there. A name that is already a directory in `~` is refused, so an unrelated
folder never gains an `stl/` subdirectory.

```bash
curl -X POST localhost:10095/api/projects -d '{"name":"gps-mount"}'
# {"name":"gps-mount","dir":"/home/rxiao/gps-mount","parts":[],...}
```

## Uploading

The ⬆ button, or dropping files anywhere on the page, saves STLs into a project's `stl/` directory, or
into a new `~/<name>/stl/` directory for a new project. A single file can be saved as a new part or as
the next version of an existing part. The page then shows it, diffed against the previous version.

An upload never replaces a file, because notes refer to files by name. A name that belongs to an
existing part becomes that part's next version (`foo.stl` → `foo_v6.stl`), unless it asks for a version
number that is still free. Files must be valid STLs of at most 256 MiB.

```bash
curl --data-binary @foo.stl 'localhost:10095/api/upload?project=cam-adapter&name=foo.stl'
# {"path":"stl/foo_v6.stl","project":"cam-adapter"}
```

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
