import {
  WebGLRenderer, Scene, PerspectiveCamera, Color, HemisphereLight, DirectionalLight, Mesh, Group,
  MeshStandardMaterial, MeshBasicMaterial, SphereGeometry, BufferGeometry, BufferAttribute,
  Line, LineBasicMaterial, GridHelper, AxesHelper, Vector3, Vector2, Plane, Raycaster, Box3, BackSide, FrontSide,
  ShaderMaterial, WebGLRenderTarget, DepthTexture, DataTexture, FloatType, RGBAFormat, NearestFilter,
  RingGeometry, DoubleSide, Sphere,
  OrbitControls, STLLoader, computeBoundsTree, disposeBoundsTree, acceleratedRaycast,
} from './vendor.js';

BufferGeometry.prototype.computeBoundsTree = computeBoundsTree;
BufferGeometry.prototype.disposeBoundsTree = disposeBoundsTree;
Mesh.prototype.raycast = acceleratedRaycast;

const $ = (id) => document.getElementById(id);
const fmt = (v, d = 2) => (Math.abs(v) < 5e-4 ? 0 : v).toFixed(d);
const fmtPt = (p) => `(${fmt(p[0])}, ${fmt(p[1])}, ${fmt(p[2])})`;
const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// ── State ────────────────────────────────────────────────────────────────────

const S = {
  projects: [],
  project: null,      // Project
  part: null,         // Part
  file: null,         // MeshFile being viewed
  cmp: null,          // MeshFile compared against, or null
  cmpMode: 'heat',
  flipped: false,     // A/B: showing cmp instead of file
  mode: 'view',
  section: { on: false, axis: 'z', flip: false, t: 0.5 },
  annotations: [],    // whole project
  showResolved: false,
  selected: null,     // annotation id
  heatRange: null,    // mm; null = auto
  stamp: { meshes: '', annotations: '' },
};

// ── Three.js scene ───────────────────────────────────────────────────────────

const stage = $('stage');
const renderer = new WebGLRenderer({ antialias: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.localClippingEnabled = true;
stage.prepend(renderer.domElement);

const scene = new Scene();
const camera = new PerspectiveCamera(35, 1, 0.1, 10000);
camera.up.set(0, 0, 1); // STL / slicer convention: Z is up
scene.add(camera);
const hemi = new HemisphereLight(0xffffff, 0x8a8f99, 1.6);
hemi.position.set(0, 0, 1);
scene.add(hemi);
const head = new DirectionalLight(0xffffff, 1.6); // headlight follows the camera
head.position.set(0.4, 0.6, 1);
camera.add(head);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.dampingFactor = 0.15;
controls.zoomToCursor = true;

const clipPlane = new Plane(new Vector3(0, 0, -1), 0);
const clipping = () => (S.section.on ? [clipPlane] : []);

const frontMat = new MeshStandardMaterial({ color: 0xc8ccd2, roughness: 0.65, metalness: 0.05, side: FrontSide });
const heatMat = new MeshStandardMaterial({ vertexColors: true, roughness: 0.7, metalness: 0, side: FrontSide });
const capMat = new MeshBasicMaterial({ color: 0xe0a800, side: BackSide }); // interior seen through a section cut (not red/blue: those are the diff colours)
const ghostMat = new MeshStandardMaterial({ color: 0x3d7fe0, transparent: true, opacity: 0.28, depthWrite: false, roughness: 0.8 });

const mainMesh = new Mesh(new BufferGeometry(), frontMat);
const capMesh = new Mesh(mainMesh.geometry, capMat);
const heatMesh = new Mesh(new BufferGeometry(), heatMat);
const ghostMesh = new Mesh(new BufferGeometry(), ghostMat);
ghostMesh.renderOrder = 2;
scene.add(mainMesh, heatMesh, capMesh, ghostMesh);

let grid = null, axes = null;
const pinGroup = new Group();
const draftGroup = new Group();
scene.add(pinGroup, draftGroup);

let bbox = new Box3(new Vector3(-10, -10, -10), new Vector3(10, 10, 10));
let pinRadius = 0.5;

function syncClipping() {
  for (const m of [frontMat, heatMat, capMat, ghostMat]) m.clippingPlanes = clipping();
  for (const ov of overlays.values()) ov.mesh.material.clippingPlanes = clipping();
  if (paint.mesh) paint.mesh.material.clippingPlanes = clipping();
}

let needsRender = true;
const requestRender = () => { needsRender = true; };
controls.addEventListener('change', () => { requestRender(); scheduleOcclusion(); });

function resize() {
  const w = stage.clientWidth, h = stage.clientHeight;
  renderer.setSize(w, h, false);
  camera.aspect = w / Math.max(h, 1);
  camera.updateProjectionMatrix();
  requestRender();
}
new ResizeObserver(resize).observe(stage);

function themeColors() {
  const cs = getComputedStyle(document.documentElement);
  scene.background = new Color(cs.getPropertyValue('--stage').trim() || '#1c2026');
  requestRender();
}
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { themeColors(); rebuildGrid(); });
themeColors();

function loop() {
  requestAnimationFrame(loop);
  if (controls.update()) needsRender = true;
  if (camAnim) stepCamAnim();
  if (!needsRender) return;
  needsRender = false;
  renderer.render(scene, camera);
  placeLabels();
}

// ── Mesh loading ─────────────────────────────────────────────────────────────

const loader = new STLLoader();
const geoCache = new Map(); // key → BufferGeometry

const meshURL = (f) => `/mesh/${encodeURIComponent(S.project.name)}/${f.path.split('/').map(encodeURIComponent).join('/')}`;

async function loadGeometry(f) {
  const key = `${S.project.name}/${f.path}@${f.mtime}:${f.size}`;
  if (geoCache.has(key)) return geoCache.get(key);
  const buf = await (await fetch(meshURL(f))).arrayBuffer();
  const geo = loader.parse(buf);
  geo.computeBoundingBox();
  geo.computeBoundsTree();
  geoCache.set(key, geo);
  if (geoCache.size > 8) {
    const [oldKey, old] = geoCache.entries().next().value;
    if (old !== mainMesh.geometry && old !== ghostMesh.geometry) { old.disposeBoundsTree(); old.dispose(); }
    geoCache.delete(oldKey);
  }
  return geo;
}

let showGeo = null; // geometry of S.file
let cmpGeo = null;  // geometry of S.cmp
let loadSeq = 0;

async function loadView({ fit = false } = {}) {
  const seq = ++loadSeq;
  busy(`Loading ${S.file.name}…`);
  try {
    const [g, c] = await Promise.all([loadGeometry(S.file), S.cmp ? loadGeometry(S.cmp) : null]);
    if (seq !== loadSeq) return;
    busy(null); // before applyDisplay, which may start the diff with its own progress
    const first = !showGeo;
    showGeo = g;
    cmpGeo = c;
    if (S.flipped && !cmpGeo) S.flipped = false;
    applyDisplay();
    bbox = showGeo.boundingBox.clone();
    if (cmpGeo) bbox.union(cmpGeo.boundingBox);
    pinRadius = Math.max(bbox.getSize(new Vector3()).length() * 0.007, 0.15);
    if (!brushSet) setBrush(bbox.getSize(new Vector3()).length() / 30, false);
    rebuildGrid();
    syncSectionRange();
    renderPins();
    if (fit || first) setView('iso');
  } catch (e) {
    if (seq === loadSeq) busy(null);
    toast(`Failed to load: ${e.message}`);
  }
}

// Decide what the scene shows from file/cmp/cmpMode/flipped.
function applyDisplay() {
  const geo = S.flipped ? cmpGeo : showGeo;
  mainMesh.geometry = geo;
  capMesh.geometry = geo;
  ghostMesh.visible = !!cmpGeo && S.cmpMode === 'ghost';
  if (ghostMesh.visible) ghostMesh.geometry = cmpGeo;
  const heat = heatWanted();
  // mainMesh stays the pick/occlusion target even while hidden behind the heat mesh
  mainMesh.visible = true;
  heatMesh.visible = false;
  if (heat) {
    computeHeat().then((entry) => {
      if (!entry || !heatWanted() || entry.src !== showGeo || entry.cmp !== cmpGeo) return;
      heatMesh.geometry = entry.geo;
      heatMesh.visible = true;
      mainMesh.visible = false;
      requestRender();
    });
  }
  $('legend').hidden = !heat;
  const fb = $('flipbtn');
  fb.hidden = !(cmpGeo && S.cmpMode === 'flip');
  fb.textContent = `Showing ${label(S.flipped ? S.cmp : S.file)} ⇄`;
  syncClipping();
  renderPins();
  renderInfo();
  updateLinks();
  requestRender();
}

const heatWanted = () => !!cmpGeo && S.cmpMode === 'heat' && !S.flipped;
const label = (f) => (f.version != null ? `v${f.version}` : f.name.replace(/\.stl$/i, ''));
const activeFile = () => (S.flipped ? S.cmp : S.file);

// ── Deviation heatmap ────────────────────────────────────────────────────────

const heatCache = new WeakMap(); // showGeo → { src, cmp, geo (refined), values (per vertex), p95, max }
const _p = new Vector3(), _a = new Vector3(), _b = new Vector3(), _c = new Vector3(), _n = new Vector3();

function faceNormal(geo, tri, out) {
  const idx = geo.index, pos = geo.attributes.position;
  const i0 = idx ? idx.getX(tri * 3) : tri * 3, i1 = idx ? idx.getX(tri * 3 + 1) : tri * 3 + 1, i2 = idx ? idx.getX(tri * 3 + 2) : tri * 3 + 2;
  _a.fromBufferAttribute(pos, i0); _b.fromBufferAttribute(pos, i1); _c.fromBufferAttribute(pos, i2);
  return out.subVectors(_c, _b).cross(_a.sub(_b)).normalize();
}

// Signed distance from p to the cmp surface: + where the shown mesh sits outside cmp
// (material added), − where it sits inside (material removed).
function signedDistance(p, geo) {
  const hit = geo.boundsTree.closestPointToPoint(p);
  if (!hit) return 0;
  faceNormal(geo, hit.faceIndex, _n);
  return Math.sign(_n.dot(_c.subVectors(p, hit.point))) * hit.distance;
}

// CAD meshes have long thin triangles; colouring their corners would smear one moved
// vertex across a whole face. Split every triangle (longest edge first) until edges are
// shorter than L, with L picked so the result stays near HEAT_TRIS triangles.
const HEAT_TRIS = 150000;
function refine(geo) {
  const pos = geo.attributes.position, idx = geo.index;
  const nTri = (idx ? idx.count : pos.count) / 3;
  const vi = (t, k) => (idx ? idx.getX(t * 3 + k) : t * 3 + k);
  let area = 0;
  for (let t = 0; t < nTri; t++) {
    _a.fromBufferAttribute(pos, vi(t, 0)); _b.fromBufferAttribute(pos, vi(t, 1)); _c.fromBufferAttribute(pos, vi(t, 2));
    area += _c.sub(_b).cross(_a.sub(_b)).length() / 2;
  }
  const diag = geo.boundingBox.getSize(new Vector3()).length();
  let L = Math.max(Math.sqrt(area / (0.433 * HEAT_TRIS)) * 1.5, diag / 1500);
  // A mesh already past the budget is dense enough not to smear -- and no edge length can
  // bring it UNDER the budget, which is what used to spin this loop forever.
  for (let tries = 0; nTri < HEAT_TRIS && tries < 12; tries++) {
    const out = split(pos, nTri, vi, L, HEAT_TRIS * 1.5);
    if (out) {
      const g = new BufferGeometry();
      g.setAttribute('position', new BufferAttribute(out.p, 3));
      g.setAttribute('normal', new BufferAttribute(out.n, 3));
      return g;
    }
    L *= 1.5; // over budget (slivers): coarser
  }
  // as-is: share the arrays, but in a geometry of its own so the colours don't land on geo
  const g = new BufferGeometry();
  g.setAttribute('position', geo.attributes.position);
  g.setAttribute('normal', geo.attributes.normal);
  if (geo.index) g.setIndex(geo.index);
  return g;
}

function split(pos, nTri, vi, L, maxTris) {
  const L2 = L * L, P = [], N = [];
  const stack = [];
  for (let t = 0; t < nTri; t++) {
    _a.fromBufferAttribute(pos, vi(t, 0)); _b.fromBufferAttribute(pos, vi(t, 1)); _c.fromBufferAttribute(pos, vi(t, 2));
    const n = _n.copy(_c).sub(_b).cross(_p.copy(_a).sub(_b)).normalize();
    stack.push([_a.x, _a.y, _a.z, _b.x, _b.y, _b.z, _c.x, _c.y, _c.z]);
    while (stack.length) {
      const q = stack.pop();
      const d01 = (q[0] - q[3]) ** 2 + (q[1] - q[4]) ** 2 + (q[2] - q[5]) ** 2;
      const d12 = (q[3] - q[6]) ** 2 + (q[4] - q[7]) ** 2 + (q[5] - q[8]) ** 2;
      const d20 = (q[6] - q[0]) ** 2 + (q[7] - q[1]) ** 2 + (q[8] - q[2]) ** 2;
      const m = Math.max(d01, d12, d20);
      if (m <= L2) {
        P.push(...q); N.push(n.x, n.y, n.z, n.x, n.y, n.z, n.x, n.y, n.z);
        if (P.length > maxTris * 9) return null;
        continue;
      }
      // rotate so the longest edge is v0–v1, then bisect it
      const r = m === d01 ? q : m === d12 ? [...q.slice(3), ...q.slice(0, 3)] : [...q.slice(6), ...q.slice(0, 6)];
      const mx = (r[0] + r[3]) / 2, my = (r[1] + r[4]) / 2, mz = (r[2] + r[5]) / 2;
      stack.push([r[0], r[1], r[2], mx, my, mz, r[6], r[7], r[8]], [mx, my, mz, r[3], r[4], r[5], r[6], r[7], r[8]]);
    }
  }
  return { p: new Float32Array(P), n: new Float32Array(N) };
}

async function computeHeat() {
  const src = showGeo, cmp = cmpGeo;
  let entry = heatCache.get(src);
  if (!entry || entry.cmp !== cmp) {
    busy('Computing diff…');
    await new Promise((r) => setTimeout(r));
    const geo = entry?.geo || refine(src);
    const pos = geo.attributes.position, n = pos.count;
    const values = new Float32Array(n);
    const seen = new Map();
    let t0 = performance.now();
    for (let i = 0; i < n; i++) {
      _p.fromBufferAttribute(pos, i);
      const key = `${_p.x},${_p.y},${_p.z}`;
      let d = seen.get(key);
      if (d === undefined) { d = signedDistance(_p, cmp); seen.set(key, d); }
      values[i] = d;
      if (performance.now() - t0 > 60) {
        busy(`Computing diff… ${Math.round((i / n) * 100)}%`);
        await new Promise((r) => setTimeout(r));
        if (showGeo !== src || cmpGeo !== cmp) return null;
        t0 = performance.now();
      }
    }
    const abs = Array.from(seen.values(), Math.abs).sort((x, y) => x - y);
    entry = { src, cmp, geo, values, p95: abs[Math.floor(abs.length * 0.95)] || 0, max: abs[abs.length - 1] || 0 };
    heatCache.set(src, entry);
    busy(null);
  }
  paintHeat(entry.geo, entry);
  return entry;
}

function paintHeat(geo, entry) {
  const R = S.heatRange ?? Math.max(0.2, Math.round(entry.p95 * 20) / 20);
  const n = entry.values.length;
  let col = geo.attributes.color;
  if (!col || col.count !== n) { col = new BufferAttribute(new Float32Array(n * 3), 3); geo.setAttribute('color', col); }
  for (let i = 0; i < n; i++) {
    const d = entry.values[i];
    const t = Math.abs(d) < 0.02 ? 0 : Math.max(-1, Math.min(1, d / R));
    // grey → red (+, added) / blue (−, removed); linear space colours
    const g = 0.56;
    if (t >= 0) col.setXYZ(i, g + (0.72 - g) * t, g * (1 - t) + 0.02 * t, g * (1 - t) + 0.02 * t);
    else col.setXYZ(i, g * (1 + t) + 0.02 * -t, g * (1 + t) + 0.1 * -t, g + (0.8 - g) * -t);
  }
  col.needsUpdate = true;
  $('legend').innerHTML = `<span>−</span><div class="ramp"></div><span>+</span>
    <span>±<input id="heatr" type="number" step="0.05" min="0.05" value="${R}"> mm</span>
    <span class="mono" title="largest deviation">max ${fmt(entry.max)}</span>`;
  $('heatr').onchange = (e) => { S.heatRange = parseFloat(e.target.value) || null; paintHeat(geo, entry); requestRender(); };
}

// ── Grid, views, camera ──────────────────────────────────────────────────────

function rebuildGrid() {
  if (grid) { scene.remove(grid); grid.geometry.dispose(); }
  if (axes) { scene.remove(axes); axes.geometry.dispose(); }
  const size = bbox.getSize(new Vector3());
  const span = Math.ceil((Math.max(size.x, size.y) * 1.6) / 10) * 10 || 50;
  const dark = matchMedia('(prefers-color-scheme: dark)').matches;
  grid = new GridHelper(span, span / 5, dark ? 0x4a525c : 0xb4bac2, dark ? 0x30363d : 0xd2d6db);
  grid.rotation.x = Math.PI / 2;
  const c = bbox.getCenter(new Vector3());
  grid.position.set(Math.round(c.x / 5) * 5, Math.round(c.y / 5) * 5, bbox.min.z - 0.01);
  axes = new AxesHelper(Math.max(size.length() * 0.15, 5)); // model origin; X red, Y green, Z blue
  scene.add(grid, axes);
  requestRender();
}

const VIEWS = {
  iso: [1, -1.3, 0.9],
  top: [0, -0.0001, 1],
  front: [0, -1, 0],
  right: [1, 0, 0],
};

function setView(name) {
  const c = bbox.getCenter(new Vector3());
  const dir = name === 'fit' ? camera.position.clone().sub(controls.target).normalize() : new Vector3(...VIEWS[name]).normalize();
  // fit the mesh's own vertices (a bounding sphere or box leaves portrait screens half empty)
  let right = new Vector3(0, 0, 1).cross(dir);
  if (right.lengthSq() < 1e-6) right.set(1, 0, 0);
  right.normalize();
  const up = dir.clone().cross(right).normalize();
  const tv = Math.tan((camera.fov * Math.PI) / 360), th = tv * camera.aspect;
  let dist = 0;
  const k = new Vector3();
  for (const g of [showGeo, cmpGeo].filter(Boolean)) {
    const pos = g.attributes.position, step = Math.max(1, Math.floor(pos.count / 30000));
    for (let i = 0; i < pos.count; i += step) {
      k.fromBufferAttribute(pos, i).sub(c);
      const z = k.dot(dir);
      dist = Math.max(dist, z + Math.abs(k.dot(right)) / th, z + Math.abs(k.dot(up)) / tv);
    }
  }
  flyTo(c.clone().addScaledVector(dir, dist * 1.1), c);
}

let camAnim = null;
function flyTo(pos, target, up = new Vector3(0, 0, 1)) {
  camAnim = { t0: performance.now(), p0: camera.position.clone(), q0: controls.target.clone(), p1: pos, q1: target, up };
}
function stepCamAnim() {
  const k = Math.min(1, (performance.now() - camAnim.t0) / 350);
  const e = k * (2 - k);
  camera.position.lerpVectors(camAnim.p0, camAnim.p1, e);
  controls.target.lerpVectors(camAnim.q0, camAnim.q1, e);
  camera.up.copy(camAnim.up);
  if (k >= 1) camAnim = null;
  controls.update();
  requestRender();
  scheduleOcclusion();
}

const cameraState = () => ({
  position: camera.position.toArray(), target: controls.target.toArray(), up: camera.up.toArray(), fov: camera.fov,
  aspect: camera.aspect,
});

// ── Section plane ────────────────────────────────────────────────────────────

function syncSectionRange() {
  const s = S.section, a = s.axis;
  const lo = bbox.min[a], hi = bbox.max[a];
  const v = lo + (hi - lo) * s.t;
  const n = new Vector3(); n[a] = s.flip ? 1 : -1;
  clipPlane.normal.copy(n);
  clipPlane.constant = s.flip ? -v : v; // keeps the side where normal·p + constant ≥ 0
  $('secval').textContent = `${a.toUpperCase()} ${s.flip ? '≥' : '≤'} ${fmt(v)} mm`;
  for (const b of $('secaxis').children) b.classList.toggle('on', b.dataset.a === a);
  syncClipping();
  requestRender();
}

// ── Picking ──────────────────────────────────────────────────────────────────

const raycaster = new Raycaster();
const ndc = new Vector2();

// Returns { point, normal, cap } on the visible surface under the pointer, honouring the
// section plane. A hit on the cut face (back face showing through the section) lands
// on the section plane itself.
function pick(ev) {
  const r = renderer.domElement.getBoundingClientRect();
  return pickNDC(((ev.clientX - r.left) / r.width) * 2 - 1, -((ev.clientY - r.top) / r.height) * 2 + 1);
}
function pickNDC(x, y) {
  ndc.set(x, y);
  raycaster.setFromCamera(ndc, camera);
  const hits = raycaster.intersectObjects([mainMesh, capMesh], false)
    .filter((h) => !S.section.on || clipPlane.distanceToPoint(h.point) >= -1e-6);
  if (!hits.length) return null;
  const h = hits[0];
  if (h.object === capMesh) {
    const p = raycaster.ray.intersectPlane(clipPlane, new Vector3());
    if (!p) return null;
    return { point: p, normal: clipPlane.normal.clone().negate(), cap: true };
  }
  return { point: h.point.clone(), normal: h.face.normal.clone(), cap: false };
}

// Taps are handled on 'click' (not pointerup) so a sheet opened by the tap can't receive
// the tap's own trailing click on its backdrop and close itself on touch screens.
let down = null;
renderer.domElement.addEventListener('pointerdown', (e) => {
  if (e.isPrimary) down = { x: e.clientX, y: e.clientY, t: performance.now() };
});
// Brush: capture phase, so OrbitControls (same element, bubble phase) sees enableRotate
// already decided for this gesture -- on the part the drag paints, on the background it rotates.
renderer.domElement.addEventListener('pointerdown', (e) => {
  if (!isBrush()) return;
  if (stroke) { endStroke(); return; } // a second finger: it's a pinch, not paint
  if (!e.isPrimary || e.button !== 0) return;
  const hit = pick(e);
  controls.enableRotate = !hit;
  if (!hit) return;
  const r = renderer.domElement.getBoundingClientRect();
  stroke = { id: e.pointerId, xy: [e.clientX - r.left, e.clientY - r.top], to: null };
  renderer.domElement.setPointerCapture(e.pointerId);
  $('chip').hidden = true;
  addDab(hit);
}, { capture: true });
renderer.domElement.addEventListener('pointermove', (e) => {
  if (!isBrush()) return;
  if (stroke && e.pointerId === stroke.id) {
    const r = renderer.domElement.getBoundingClientRect();
    stroke.to = [e.clientX - r.left, e.clientY - r.top];
    if (!strokeQueued) { strokeQueued = true; requestAnimationFrame(stepStroke); }
  } else if (!stroke && e.pointerType === 'mouse' && e.buttons === 0) {
    showRing(pick(e));
  }
});
const strokeEnd = (e) => {
  if (stroke && e.pointerId === stroke.id) endStroke();
  if (isBrush()) controls.enableRotate = true;
};
renderer.domElement.addEventListener('pointerup', strokeEnd);
renderer.domElement.addEventListener('pointercancel', strokeEnd);
renderer.domElement.addEventListener('pointerleave', () => showRing(null));
renderer.domElement.addEventListener('click', (e) => {
  if (!down) return;
  const moved = Math.hypot(e.clientX - down.x, e.clientY - down.y);
  const quick = performance.now() - down.t < 600;
  down = null;
  if (moved < 6 && quick) onTap(e);
});

function onTap(ev) {
  if (isBrush()) return; // a tap already left a dab on pointerdown
  const hit = pick(ev);
  if (S.mode === 'measure') {
    if (hit) measureTap(hit);
    return;
  }
  // view mode: report coordinates (and deviation when diffing)
  clearDraft();
  if (!hit) { $('chip').hidden = true; return; }
  addDraftDot(hit.point);
  let txt = `<span class="mono">${fmtPt(hit.point.toArray())}${hit.cap ? ' · section' : ''}</span>`;
  if (cmpGeo && S.cmpMode === 'heat' && !S.flipped) {
    txt += `<span class="mono">Δ ${fmt(signedDistance(hit.point, cmpGeo))} mm vs ${label(S.cmp)}</span>`;
  }
  chip(txt + `<button id="chipnote">Note here</button>`);
  $('chipnote').onclick = () => startPin(hit);
}

// ── Draft markers (pending pin, measurement, tapped point) ───────────────────

function clearDraft() {
  for (const o of [...draftGroup.children]) { draftGroup.remove(o); o.geometry?.dispose(); }
  draftLabels = [];
  requestRender();
}
let draftLabels = [];
function addDraftDot(p, color = 0xe8711a) {
  const m = new Mesh(new SphereGeometry(pinRadius * 0.8, 16, 12), new MeshBasicMaterial({ color, depthTest: false }));
  m.position.copy(p);
  m.renderOrder = 10;
  draftGroup.add(m);
  requestRender();
}
function addDraftLine(a, b, color = 0xe8711a) {
  const g = new BufferGeometry().setFromPoints([a, b]);
  const l = new Line(g, new LineBasicMaterial({ color, depthTest: false }));
  l.renderOrder = 10;
  draftGroup.add(l);
}

let measure = [];
function measureTap(hit) {
  if (measure.length >= 2) { measure = []; clearDraft(); }
  measure.push(hit);
  addDraftDot(hit.point);
  if (measure.length === 1) { chip(`<span>Tap the second point</span>`); return; }
  const [a, b] = measure.map((h) => h.point);
  addDraftLine(a, b);
  const d = b.clone().sub(a);
  draftLabels = [{ pos: a.clone().add(b).multiplyScalar(0.5), text: `${fmt(d.length())} mm` }];
  chip(`<span class="mono"><b>${fmt(d.length())} mm</b> · Δx ${fmt(d.x)} Δy ${fmt(d.y)} Δz ${fmt(d.z)}</span>
    <button id="mnote">Note</button><button id="mclear">Clear</button>`);
  $('mnote').onclick = () => startPin(measure[0], measure[1]);
  $('mclear').onclick = () => { measure = []; clearDraft(); $('chip').hidden = true; };
  requestRender();
}

// ── Paint brush ──────────────────────────────────────────────────────────────
//
// A painted selection is an ordered list of dabs [x, y, z, ±r, nx, ny, nz] in model mm: a
// sphere of radius r at a surface point, + paints and − erases, with the surface normal
// there. A surface point p (normal n) is painted when the LAST dab with |p − c| < r and
// n·n_c > 0 is positive. The normal test keeps the far side of a thin wall clean; the order
// lets you erase and repaint. Dabs are plain model geometry, so a painted note re-renders
// on any version of the part.

const MAX_DABS = 4096;
const DAB_ROW = 64;                         // dabs per texture row, 2 texels each
const paint = { dabs: [], tris: new Set(), geo: null, mesh: null, tex: null };
let brushD = 3;                             // brush diameter, mm
let brushSet = false;                       // true once the user (or storage) chose a size
let brushErase = false;
let stroke = null;                          // { id, xy: [px, py], to: [px, py] } in canvas px
let strokeQueued = false;
const isBrush = () => S.mode === 'brush';

try {
  const v = parseFloat(localStorage.getItem('stlr.brushD'));
  if (v > 0) { brushD = v; brushSet = true; }
} catch {}

function setBrush(d, remember = true) {
  brushD = Math.min(20, Math.max(0.5, Math.round(d * 2) / 2));
  $('brushsize').value = brushD;
  $('brushval').textContent = `${brushD.toFixed(1)} mm`;
  ring.scale.setScalar(brushD / 2);
  if (remember) { brushSet = true; try { localStorage.setItem('stlr.brushD', brushD); } catch {} }
  requestRender();
}

// footprint preview: a ring lying on the surface under the cursor
const ring = new Mesh(new RingGeometry(0.88, 1, 48),
  new MeshBasicMaterial({ color: 0xff7a00, transparent: true, opacity: 0.9, depthTest: false, side: DoubleSide }));
ring.renderOrder = 11;
ring.visible = false;
ring.raycast = () => {};
scene.add(ring);
const _zAxis = new Vector3(0, 0, 1);
function showRing(hit) {
  ring.visible = !!hit && isBrush();
  if (hit) {
    ring.position.copy(hit.point).addScaledVector(hit.normal, 0.02);
    ring.quaternion.setFromUnitVectors(_zAxis, hit.normal);
    ring.material.color.set(brushErase ? 0x3d7fe0 : 0xff7a00);
  }
  requestRender();
}

function addDab(hit) {
  if (paint.dabs.length >= MAX_DABS) { toast('This painting is full: add the note, then start another'); return; }
  const r = brushD / 2, p = hit.point, n = hit.normal;
  const dab = [p.x, p.y, p.z, brushErase ? -r : r, n.x, n.y, n.z];
  paint.dabs.push(dab);
  if (dab[3] > 0 && paint.geo) trisNear(paint.geo, dab, paint.tris);
  stroke && (stroke.last = p.clone());
  showRing(hit);
  queueDraft();
}

// Fill the gap between pointer events with dabs every r/3 along the surface, sampling the
// screen path in 4 px steps so a fast stroke still paints a continuous band.
function stepStroke() {
  strokeQueued = false;
  if (!stroke || !stroke.to) return;
  const [x0, y0] = stroke.xy, [x1, y1] = stroke.to;
  const w = renderer.domElement.clientWidth, h = renderer.domElement.clientHeight;
  const n = Math.min(40, Math.max(1, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / 4)));
  for (let i = 1; i <= n; i++) {
    const x = x0 + ((x1 - x0) * i) / n, y = y0 + ((y1 - y0) * i) / n;
    const hit = pickNDC((x / w) * 2 - 1, 1 - (y / h) * 2);
    if (!hit) continue;
    if (!stroke.last || hit.point.distanceTo(stroke.last) >= brushD / 6) addDab(hit);
    else showRing(hit);
  }
  stroke.xy = stroke.to;
}

function endStroke() {
  stroke = null;
  updatePaintReadout();
}

function clearPaint() {
  paint.dabs = [];
  paint.tris = new Set();
  queueDraft();
  updatePaintReadout();
}

function updatePaintReadout() {
  const reg = paint.dabs.length ? paintRegion(paint.dabs, mainMesh.geometry) : null;
  $('paintarea').textContent = reg ? `~${fmt(reg.area_mm2, reg.area_mm2 < 10 ? 1 : 0)} mm² painted` : '';
  $('paintnote').disabled = !reg;
  $('paintclear').disabled = !paint.dabs.length;
}

// triangles of geo touched by one dab's sphere (BVH shapecast; indices into geo.index)
const _sph = new Sphere(), _cp = new Vector3();
function trisNear(geo, dab, out) {
  _sph.center.set(dab[0], dab[1], dab[2]);
  _sph.radius = Math.abs(dab[3]);
  const r2 = _sph.radius * _sph.radius;
  geo.boundsTree.shapecast({
    intersectsBounds: (box) => box.intersectsSphere(_sph),
    intersectsTriangle: (tri, i) => {
      if (tri.closestPointToPoint(_sph.center, _cp).distanceToSquared(_sph.center) < r2) out.add(i);
      return false;
    },
  });
  return out;
}

const paintFrag = `
  #include <clipping_planes_pars_fragment>
  precision highp sampler2D;
  uniform sampler2D dabs;
  uniform int count;
  uniform vec3 color;
  uniform float opacity;
  varying vec3 vWorld;
  varying vec3 vNormalW;
  void main() {
    #include <clipping_planes_fragment>
    vec3 n = normalize(vNormalW);
    bool on = false;
    for (int i = 0; i < ${MAX_DABS}; i++) {
      if (i >= count) break;
      ivec2 px = ivec2((i % ${DAB_ROW}) * 2, i / ${DAB_ROW});
      vec4 a = texelFetch(dabs, px, 0);
      if (distance(vWorld, a.xyz) < abs(a.w) && dot(n, texelFetch(dabs, px + ivec2(1, 0), 0).xyz) > 0.0)
        on = a.w > 0.0;
    }
    if (!on) discard;
    gl_FragColor = vec4(color, opacity);
  }`;

function dabTexture(dabs) {
  const tex = new DataTexture(new Float32Array(DAB_ROW * 2 * (MAX_DABS / DAB_ROW) * 4),
    DAB_ROW * 2, MAX_DABS / DAB_ROW, RGBAFormat, FloatType);
  tex.minFilter = tex.magFilter = NearestFilter;
  writeDabs(tex, dabs);
  return tex;
}
function writeDabs(tex, dabs) {
  const d = tex.image.data;
  for (let i = 0; i < dabs.length; i++) {
    const o = (Math.floor(i / DAB_ROW) * DAB_ROW * 2 + (i % DAB_ROW) * 2) * 4, a = dabs[i];
    d[o] = a[0]; d[o + 1] = a[1]; d[o + 2] = a[2]; d[o + 3] = a[3];
    d[o + 4] = a[4]; d[o + 5] = a[5]; d[o + 6] = a[6]; d[o + 7] = 0;
  }
  tex.needsUpdate = true;
}
function paintMaterial(tex, count) {
  const mat = new ShaderMaterial({
    vertexShader: projVert, fragmentShader: paintFrag, transparent: true, depthWrite: false, clipping: true,
    polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -4,
    uniforms: { dabs: { value: tex }, count: { value: count }, color: { value: new Color(0xff7a00) }, opacity: { value: 0.45 } },
  });
  mat.clippingPlanes = clipping();
  return mat;
}

// The candidate triangles, copied out (not indexed into the shared buffers, so the overlay
// can be disposed without taking the main mesh's GPU buffers with it).
function subsetGeometry(geo, tris) {
  const P = new Float32Array(tris.size * 9), N = new Float32Array(tris.size * 9);
  const pos = geo.attributes.position, nor = geo.attributes.normal, idx = geo.index;
  let k = 0;
  for (const t of tris) {
    for (let j = 0; j < 3; j++) {
      const v = idx ? idx.getX(t * 3 + j) : t * 3 + j;
      P[k] = pos.getX(v); P[k + 1] = pos.getY(v); P[k + 2] = pos.getZ(v);
      N[k] = nor.getX(v); N[k + 1] = nor.getY(v); N[k + 2] = nor.getZ(v);
      k += 3;
    }
  }
  const g = new BufferGeometry();
  g.setAttribute('position', new BufferAttribute(P, 3));
  g.setAttribute('normal', new BufferAttribute(N, 3));
  return g;
}

function buildPaintOverlay(dabs, geo) {
  const tris = new Set();
  for (const d of dabs) if (d[3] > 0) trisNear(geo, d, tris);
  const tex = dabTexture(dabs);
  const mesh = new Mesh(subsetGeometry(geo, tris), paintMaterial(tex, dabs.length));
  mesh.renderOrder = 3;
  mesh.raycast = () => {}; // never pickable
  scene.add(mesh);
  return {
    geo, mesh,
    dispose() { scene.remove(mesh); mesh.geometry.dispose(); mesh.material.dispose(); tex.dispose(); },
  };
}

// the in-progress painting, rebuilt at most once per frame while strokes come in
let draftQueued = false;
function queueDraft() {
  if (draftQueued) return;
  draftQueued = true;
  requestAnimationFrame(() => { draftQueued = false; syncDraft(); });
}
function syncDraft() {
  const geo = mainMesh.geometry;
  if (!geo.boundsTree) return;
  if (paint.geo !== geo) { // version switch / A-B: the same dabs, re-found on this surface
    paint.geo = geo;
    paint.tris = new Set();
    for (const d of paint.dabs) if (d[3] > 0) trisNear(geo, d, paint.tris);
  }
  if (!paint.mesh) {
    paint.tex = dabTexture([]);
    paint.mesh = new Mesh(new BufferGeometry(), paintMaterial(paint.tex, 0));
    paint.mesh.renderOrder = 4;
    paint.mesh.raycast = () => {};
    scene.add(paint.mesh);
  }
  paint.mesh.geometry.dispose();
  paint.mesh.geometry = subsetGeometry(geo, paint.tris);
  writeDabs(paint.tex, paint.dabs);
  paint.mesh.material.uniforms.count.value = paint.dabs.length;
  paint.mesh.visible = paint.dabs.length > 0;
  requestRender();
}

// What a painted selection covers, in model mm, for the note: sample each candidate
// triangle (count ∝ area, fixed seed so the numbers are stable) and keep the samples the
// dab rule paints.
function paintedAt(p, n, dabs) {
  let on = false;
  for (const a of dabs) {
    const dx = p.x - a[0], dy = p.y - a[1], dz = p.z - a[2];
    if (dx * dx + dy * dy + dz * dz < a[3] * a[3] && n.x * a[4] + n.y * a[5] + n.z * a[6] > 0) on = a[3] > 0;
  }
  return on;
}
function paintRegion(dabs, geo) {
  if (!geo.boundsTree) return null;
  const tris = new Set();
  for (const d of dabs) if (d[3] > 0) trisNear(geo, d, tris);
  if (!tris.size) return null;
  const pos = geo.attributes.position, idx = geo.index;
  const T = [];
  let total = 0;
  for (const t of tris) {
    const v = [0, 1, 2].map((j) => new Vector3().fromBufferAttribute(pos, idx ? idx.getX(t * 3 + j) : t * 3 + j));
    const n = new Vector3().subVectors(v[1], v[0]).cross(new Vector3().subVectors(v[2], v[0]));
    const A = n.length() / 2;
    if (A < 1e-9) continue;
    T.push({ v, n: n.normalize(), A });
    total += A;
  }
  const cell = total / 6000;
  let seed = 12345;
  const rnd = () => (seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296;
  const pts = [], c = new Vector3(), nsum = new Vector3(), box = new Box3();
  let area = 0;
  for (const { v, n, A } of T) {
    const k = Math.min(200, Math.max(1, Math.round(A / cell)));
    for (let j = 0; j < k; j++) {
      let u = rnd(), w = rnd();
      if (u + w > 1) { u = 1 - u; w = 1 - w; }
      const p = v[0].clone().addScaledVector(_cp.subVectors(v[1], v[0]), u).addScaledVector(_p.subVectors(v[2], v[0]), w);
      if (!paintedAt(p, n, dabs)) continue;
      area += A / k;
      c.addScaledVector(p, A / k);
      nsum.addScaledVector(n, A / k);
      box.expandByPoint(p);
      pts.push(p);
    }
  }
  if (!pts.length) return null;
  c.divideScalar(area);
  nsum.normalize();
  const anchor = pts.reduce((a, p) => (p.distanceToSquared(c) < a.distanceToSquared(c) ? p : a));
  const r2 = (v) => v.toArray().map((q) => Math.round(q * 100) / 100);
  const r3 = (q) => Math.round(q * 1000) / 1000;
  const step = Math.max(1, Math.ceil(pts.length / 250));
  return {
    shape: 'paint', brush: { dabs: dabs.map((d) => d.map(r3)) },
    centroid: r2(c), size: r2(box.getSize(new Vector3())), bbox: { min: r2(box.min), max: r2(box.max) },
    normal: r2(nsum), area_mm2: Math.round(area * 10) / 10, sample_count: pts.length,
    samples: pts.filter((_, k) => k % step === 0).map(r2),
    section: S.section.on ? $('secval').textContent : undefined,
    anchor: { point: anchor },
  };
}

// ── Area highlight: project each area note back onto the surface ────────────
//
// The shape is re-projected from the camera it was drawn with, like a slide projector
// with a shadow map: a fragment is lit when it falls inside the shape, faces that camera,
// and nothing sat in front of it. Works on any version since it only needs the camera.

const overlays = new Map(); // note id → { geo, mesh, dispose } (projector or paint)
const depthMat = new MeshBasicMaterial({ colorWrite: false });
const depthScene = new Scene();
const depthMesh = new Mesh(new BufferGeometry(), depthMat);
depthScene.add(depthMesh);

const projVert = `
  #include <clipping_planes_pars_vertex>
  varying vec3 vWorld;
  varying vec3 vNormalW;
  void main() {
    vec4 wp = modelMatrix * vec4(position, 1.0);
    vWorld = wp.xyz;
    vNormalW = normalize(mat3(modelMatrix) * normal);
    vec4 mvPosition = viewMatrix * wp;
    gl_Position = projectionMatrix * mvPosition;
    #include <clipping_planes_vertex>
  }`;
const projFrag = `
  #include <clipping_planes_pars_fragment>
  uniform mat4 pView, pProj;
  uniform sampler2D depthTex;
  uniform vec4 shape;
  uniform float isRect, pNear, pFar, bias, opacity;
  uniform vec3 color, pCamPos;
  varying vec3 vWorld;
  varying vec3 vNormalW;
  void main() {
    #include <clipping_planes_fragment>
    vec4 v = pView * vec4(vWorld, 1.0);
    vec4 c = pProj * v;
    if (c.w <= 0.0) discard;
    vec3 n = c.xyz / c.w;
    vec2 d = (n.xy - shape.xy) / shape.zw;
    float r = isRect > 0.5 ? max(abs(d.x), abs(d.y)) : length(d);
    if (r > 1.0) discard;
    // surfaces seen nearly edge-on from the drawing camera weren't really visible there
    float facing = dot(normalize(vNormalW), normalize(pCamPos - vWorld));
    if (facing < 0.08) discard;
    float zn = texture2D(depthTex, n.xy * 0.5 + 0.5).x * 2.0 - 1.0;
    float sceneZ = 2.0 * pNear * pFar / (pFar + pNear - zn * (pFar - pNear));
    if (-v.z > sceneZ + bias / max(facing, 0.15)) discard; // slope-scaled: no acne on oblique faces
    gl_FragColor = vec4(color, mix(opacity, 0.95, smoothstep(0.88, 0.96, r)));
  }`;

function syncOverlays(notes) {
  const geo = mainMesh.geometry;
  if (!geo.boundingBox) return; // mesh not loaded yet; renderPins runs again once it is
  syncDraft();
  const want = new Set();
  for (const a of notes) {
    const isPaint = a.region?.shape === 'paint';
    if (!a.region || (!isPaint && !a.camera)) continue;
    want.add(a.id);
    let ov = overlays.get(a.id);
    if (!ov || ov.geo !== geo) {
      ov?.dispose();
      ov = isPaint ? buildPaintOverlay(a.region.brush.dabs, geo) : buildProjector(a, geo);
      overlays.set(a.id, ov);
    }
    const done = a.status !== 'open';
    ov.mesh.material.uniforms.color.value.set(done ? 0x2f9e55 : 0xff7a00);
    ov.mesh.material.uniforms.opacity.value = a.id === S.selected ? 0.55 : 0.35;
  }
  for (const [id, ov] of [...overlays]) if (!want.has(id)) { ov.dispose(); overlays.delete(id); }
}

function buildProjector(a, geo) {
  const cam = a.camera, s = a.region.ndc;
  const aspect = cam.aspect || 1;
  const pos = new Vector3(...cam.position), target = new Vector3(...cam.target);
  const c = geo.boundingBox.getCenter(new Vector3()), R = geo.boundingBox.getSize(new Vector3()).length() / 2;
  const D = pos.distanceTo(c);
  const pcam = new PerspectiveCamera(cam.fov, aspect, Math.max(0.05, D - R * 1.2), D + R * 1.2);
  pcam.position.copy(pos);
  pcam.up.set(...(cam.up || [0, 0, 1]));
  pcam.lookAt(target);
  pcam.updateMatrixWorld();
  pcam.updateProjectionMatrix();
  const W = 2048, H = Math.max(64, Math.round(2048 / aspect));
  const rt = new WebGLRenderTarget(W, H);
  rt.depthTexture = new DepthTexture(W, H);
  depthMesh.geometry = geo;
  const prevClip = renderer.localClippingEnabled;
  renderer.localClippingEnabled = false;
  renderer.setRenderTarget(rt);
  renderer.clear();
  renderer.render(depthScene, pcam);
  renderer.setRenderTarget(null);
  renderer.localClippingEnabled = prevClip;
  const mat = new ShaderMaterial({
    vertexShader: projVert, fragmentShader: projFrag, transparent: true, depthWrite: false, clipping: true,
    polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -4,
    uniforms: {
      pView: { value: pcam.matrixWorldInverse.clone() }, pProj: { value: pcam.projectionMatrix.clone() },
      depthTex: { value: rt.depthTexture }, shape: { value: [s.cx, s.cy, s.hx, s.hy] },
      isRect: { value: a.region.shape === 'rect' ? 1 : 0 }, pNear: { value: pcam.near }, pFar: { value: pcam.far },
      bias: { value: R * 0.008 }, opacity: { value: 0.35 }, color: { value: new Color(0xff7a00) }, pCamPos: { value: pos },
    },
  });
  mat.clippingPlanes = clipping();
  const mesh = new Mesh(geo, mat);
  mesh.renderOrder = 3;
  mesh.raycast = () => {}; // never pickable
  scene.add(mesh);
  return {
    geo, mesh,
    dispose() { scene.remove(mesh); mat.dispose(); rt.depthTexture.dispose(); rt.dispose(); },
  };
}

// ── Notes (annotations) ──────────────────────────────────────────────────────

let pending = null; // { hit, hit2, region, editId }

function startPin(hit, hit2 = null) {
  pending = { hit, hit2 };
  clearDraft();
  addDraftDot(hit.point);
  if (hit2) { addDraftDot(hit2.point); addDraftLine(hit.point, hit2.point); }
  const f = activeFile();
  const len = hit2 ? ` → ${fmtPt(hit2.point.toArray())} = ${fmt(hit.point.distanceTo(hit2.point))} mm` : '';
  openSheet(`${label(f)} · ${fmtPt(hit.point.toArray())}${len}`, '');
}

function openSheet(title, text) {
  $('sheettitle').textContent = title;
  $('sheettext').value = text;
  $('sheet').hidden = false;
  setTimeout(() => $('sheettext').focus(), 50);
}
function closeSheet() {
  $('sheet').hidden = true;
  pending = null;
  if (S.mode !== 'measure') clearDraft();
}

$('sheetcancel').onclick = closeSheet;
$('sheet').onclick = (e) => { if (e.target === $('sheet')) closeSheet(); };
$('sheettext').onkeydown = (e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) $('sheetsave').click(); };
$('sheetsave').onclick = async () => {
  const body = $('sheettext').value.trim();
  const p = pending;
  if (!p) return;
  if (p.editId) {
    await patchAnnotation(p.editId, { body });
    closeSheet();
    return;
  }
  const f = activeFile();
  const shot = screenshot(p, body);
  let req;
  if (p.region) {
    const { anchor, ...region } = p.region;
    req = {
      project: S.project.name, file: f.path, kind: region.shape,
      point: anchor.point.toArray(), normal: region.normal, region,
      body, camera: cameraState(), screenshot: shot,
    };
  } else {
    req = {
      project: S.project.name, file: f.path, kind: p.hit2 ? 'measure' : 'pin',
      point: p.hit.point.toArray(), normal: p.hit.normal.toArray(),
      point2: p.hit2 ? p.hit2.point.toArray() : null,
      body, camera: cameraState(), screenshot: shot,
    };
  }
  const res = await fetch('/api/annotations', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(req) });
  if (!res.ok) { toast(`Save failed: ${await res.text()}`); return; }
  const n = await res.json();
  S.annotations.push(n);
  S.selected = n.id;
  measure = [];
  if (p.region?.shape === 'paint') clearPaint();
  $('chip').hidden = true;
  closeSheet();
  renderPins();
  toast(`Saved note #${n.id}`);
};

// Renders the current view with the draft markers and burns in a caption, so the PNG
// is self-explanatory when read later without the app.
function screenshot(p, body) {
  ring.visible = false;
  renderer.render(scene, camera);
  const src = renderer.domElement;
  const scale = Math.min(1, 1400 / Math.max(src.width, src.height));
  const w = Math.round(src.width * scale), h = Math.round(src.height * scale);
  const cv = document.createElement('canvas');
  cv.width = w; cv.height = h + 54;
  const g = cv.getContext('2d');
  g.drawImage(src, 0, 0, w, h);
  const proj = (v) => { const q = v.clone().project(camera); return [(q.x + 1) / 2 * w, (1 - q.y) / 2 * h]; };
  g.strokeStyle = '#ff7a00'; g.lineWidth = 3;
  if (p.region?.ndc) {
    const s = p.region.ndc;
    const x = ((s.cx + 1) / 2) * w, y = ((1 - s.cy) / 2) * h, rx = (s.hx / 2) * w, ry = (s.hy / 2) * h;
    g.beginPath();
    if (p.region.shape === 'rect') g.rect(x - rx, y - ry, 2 * rx, 2 * ry); else g.ellipse(x, y, rx, ry, 0, 0, Math.PI * 2);
    g.stroke();
  }
  for (const pt of [p.hit, p.hit2].filter(Boolean)) {
    const [x, y] = proj(pt.point);
    g.beginPath(); g.arc(x, y, 18, 0, Math.PI * 2); g.stroke();
    g.beginPath(); g.moveTo(x - 28, y); g.lineTo(x - 10, y); g.moveTo(x + 10, y); g.lineTo(x + 28, y);
    g.moveTo(x, y - 28); g.lineTo(x, y - 10); g.moveTo(x, y + 10); g.lineTo(x, y + 28); g.stroke();
  }
  g.fillStyle = '#111'; g.fillRect(0, h, w, 54);
  g.fillStyle = '#fff'; g.font = '14px ui-monospace, monospace';
  const f = activeFile();
  let line1 = p.region
    ? `${S.project.name}/${f.path} ${p.region.shape} ${p.region.size.map((v) => fmt(v, 1)).join('×')} mm ~${fmt(p.region.area_mm2, 0)} mm² around ${fmtPt(p.region.centroid)}`
    : `${S.project.name}/${f.path} @ ${fmtPt(p.hit.point.toArray())}`;
  if (p.hit2) line1 += ` → ${fmtPt(p.hit2.point.toArray())} = ${fmt(p.hit.point.distanceTo(p.hit2.point))} mm`;
  if (S.section.on) line1 += `  [section ${$('secval').textContent}]`;
  if (cmpGeo && !S.flipped && S.cmpMode !== 'flip') line1 += `  [${S.cmpMode} vs ${label(S.cmp)}]`;
  g.fillText(line1, 10, h + 20);
  g.fillText(body.replace(/\s+/g, ' ').slice(0, Math.floor(w / 8.5)), 10, h + 42);
  requestRender();
  return cv.toDataURL('image/png');
}

async function patchAnnotation(id, fields) {
  const res = await fetch(`/api/annotations/${id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(fields) });
  if (!res.ok) { toast(`Update failed: ${await res.text()}`); return; }
  const n = await res.json();
  S.annotations = S.annotations.map((a) => (a.id === id ? n : a));
  renderPins();
}

async function fetchAnnotations() {
  if (!S.project) return;
  const res = await fetch(`/api/annotations?project=${encodeURIComponent(S.project.name)}`);
  S.annotations = await res.json();
  renderPins();
}

// Notes for the current part, across all of its versions — a pin left on v13 is still
// about the same feature on v14 because every version shares the model's coordinate frame.
function visibleNotes() {
  if (!S.part) return [];
  return S.annotations.filter((a) => a.part === S.part.name && (S.showResolved || a.status === 'open'));
}

let pinLabels = []; // { el, pos, id }
function renderPins() {
  for (const o of [...pinGroup.children]) { pinGroup.remove(o); o.geometry?.dispose(); }
  $('labels').querySelectorAll('.label').forEach((e) => e.remove());
  pinLabels = [];
  const notes = visibleNotes();
  const f = activeFile();
  for (const a of notes) {
    const done = a.status !== 'open';
    const color = done ? 0x2f9e55 : 0xe8711a;
    const p = new Vector3(...a.point);
    const m = new Mesh(new SphereGeometry(pinRadius, 16, 12), new MeshBasicMaterial({ color }));
    m.position.copy(p);
    if (!a.region) pinGroup.add(m); // an area note is marked by its projected highlight
    let lpos = p;
    if (a.point2) {
      const q = new Vector3(...a.point2);
      const m2 = m.clone(); m2.geometry = new SphereGeometry(pinRadius, 16, 12); m2.position.copy(q);
      pinGroup.add(m2);
      const l = new Line(new BufferGeometry().setFromPoints([p, q]), new LineBasicMaterial({ color }));
      pinGroup.add(l);
      lpos = p.clone().add(q).multiplyScalar(0.5);
    }
    const el = document.createElement('div');
    el.className = 'label' + (done ? ' done' : '') + (f && a.file !== f.path ? ' other' : '') + (a.id === S.selected ? ' sel' : '');
    el.textContent = a.point2 ? `#${a.id} ${fmt(new Vector3(...a.point).distanceTo(new Vector3(...a.point2)))}mm` : `#${a.id}`;
    el.title = a.body;
    el.onclick = () => selectNote(a.id, false);
    $('labels').append(el);
    pinLabels.push({ el, pos: lpos, id: a.id });
  }
  syncOverlays(notes);
  renderPinList(notes);
  scheduleOcclusion();
  requestRender();
}

function renderPinList(notes) {
  const open = S.part ? S.annotations.filter((a) => a.part === S.part.name && a.status === 'open').length : 0;
  $('pincount').textContent = open ? `(${open})` : '';
  const f = activeFile();
  if (!notes.length) {
    $('pins').innerHTML = `<div class="empty">No ${S.showResolved ? '' : 'open '}notes on this part. Choose <b>Brush</b>, paint the area on the part, then <b>Add note</b>.</div>`;
    return;
  }
  $('pins').innerHTML = notes.slice().reverse().map((a) => {
    const ver = a.version != null ? `v${a.version}` : a.file.split('/').pop();
    const where = a.region
      ? `${a.region.shape} ${a.region.size.map((v) => fmt(v, 1)).join('×')} mm · ~${fmt(a.region.area_mm2, 0)} mm² around ${fmtPt(a.region.centroid)}`
      : a.point2 ? `${fmt(new Vector3(...a.point).distanceTo(new Vector3(...a.point2)))} mm` : fmtPt(a.point);
    return `<div class="pin${a.id === S.selected ? ' sel' : ''}${a.status !== 'open' ? ' done' : ''}" data-id="${a.id}">
      <div class="pin-top"><span class="num">#${a.id}</span><span>${esc(ver)}${f && a.file !== f.path ? ' (other version)' : ''}</span>
        <span class="badge ${a.status}">${a.status}${a.resolved_in ? ' in ' + esc(a.resolved_in.split('/').pop()) : ''}</span></div>
      <div class="pin-body">${esc(a.body) || '<i>(no text)</i>'}</div>
      ${a.reply ? `<div class="pin-reply">${esc(a.reply)}</div>` : ''}
      <div class="pin-top mono">${where}</div>
      <div class="pin-actions">
        <button data-act="status">${a.status === 'open' ? 'Resolve' : 'Reopen'}</button>
        <button data-act="edit">Edit</button>
        ${a.screenshot ? `<a class="btn" href="${a.screenshot}" target="_blank">Shot</a>` : ''}
        <button data-act="delete">Delete</button>
      </div></div>`;
  }).join('');
}

$('pins').onclick = async (e) => {
  const card = e.target.closest('.pin');
  if (!card) return;
  const id = +card.dataset.id;
  const a = S.annotations.find((x) => x.id === id);
  const act = e.target.dataset.act;
  if (act === 'status') { await patchAnnotation(id, { status: a.status === 'open' ? 'resolved' : 'open' }); return; }
  if (act === 'edit') { pending = { editId: id }; openSheet(`Edit note #${id}`, a.body); return; }
  if (act === 'delete') {
    if (!confirm(`Delete note #${id}?`)) return;
    await fetch(`/api/annotations/${id}`, { method: 'DELETE' });
    S.annotations = S.annotations.filter((x) => x.id !== id);
    renderPins();
    return;
  }
  if (e.target.closest('a')) return;
  selectNote(id, true);
};

function selectNote(id, fly) {
  S.selected = id;
  const a = S.annotations.find((x) => x.id === id);
  renderPins();
  document.querySelector(`.pin[data-id="${id}"]`)?.scrollIntoView({ block: 'nearest' });
  if (fly && a?.camera) {
    flyTo(new Vector3(...a.camera.position), new Vector3(...a.camera.target), new Vector3(...(a.camera.up || [0, 0, 1])));
    document.body.classList.remove('panel-open');
  } else if (!fly) {
    document.body.classList.add('panel-open');
  }
}

function placeLabels() {
  const w = stage.clientWidth, h = stage.clientHeight;
  const all = [...pinLabels, ...draftLabels.map((d) => {
    if (!d.el) { d.el = document.createElement('div'); d.el.className = 'label measure'; d.el.textContent = d.text; $('labels').append(d.el); }
    return d;
  })];
  for (const l of all) {
    const q = l.pos.clone().project(camera);
    const vis = q.z < 1 && Math.abs(q.x) < 1.2 && Math.abs(q.y) < 1.2;
    l.el.style.display = vis ? '' : 'none';
    if (vis) { l.el.style.left = `${(q.x + 1) / 2 * w}px`; l.el.style.top = `${(1 - q.y) / 2 * h}px`; }
  }
  $('labels').querySelectorAll('.label.measure').forEach((el) => { if (!draftLabels.some((d) => d.el === el)) el.remove(); });
}

// Dim labels whose pin is hidden behind the model; run only when the camera settles.
let occTimer = null;
function scheduleOcclusion() {
  clearTimeout(occTimer);
  occTimer = setTimeout(() => {
    const rc = new Raycaster();
    for (const l of pinLabels) {
      const dir = l.pos.clone().sub(camera.position);
      const dist = dir.length();
      rc.set(camera.position, dir.normalize());
      rc.far = dist - pinRadius * 1.5;
      const hits = rc.intersectObject(mainMesh, false).filter((h) => !S.section.on || clipPlane.distanceToPoint(h.point) >= 0);
      l.el.classList.toggle('occluded', hits.length > 0);
    }
  }, 120);
}

// ── Info, links, UI chrome ───────────────────────────────────────────────────

function renderInfo() {
  const f = activeFile();
  const g = mainMesh.geometry;
  if (!f || !g.boundingBox) return;
  const s = g.boundingBox.getSize(new Vector3());
  const when = new Date(f.mtime).toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
  $('info').innerHTML = `<b>${esc(f.name)}</b><br>
    <span class="mono">${fmt(s.x, 1)} × ${fmt(s.y, 1)} × ${fmt(s.z, 1)} mm</span><br>
    ${(g.attributes.position.count / 3).toLocaleString()} tris · ${(f.size / 1024).toFixed(0)} KB · ${when}`;
}

function updateLinks() {
  const f = activeFile();
  if (!f) return;
  const url = new URL(meshURL(f), location.href).href;
  $('download').href = url;
  $('download').setAttribute('download', f.name);
  $('bambu').href = `bambustudio://open?file=${encodeURIComponent(url)}`;
}

function chip(html) { $('chip').innerHTML = html; $('chip').hidden = false; }
let toastTimer;
function toast(msg) {
  $('toast').textContent = msg; $('toast').hidden = false;
  clearTimeout(toastTimer); toastTimer = setTimeout(() => { $('toast').hidden = true; }, 3500);
}
function busy(msg) { $('busy').hidden = !msg; if (msg) $('busy').textContent = msg; }

function setMode(m) {
  S.mode = m;
  for (const b of $('mode').children) b.classList.toggle('on', b.dataset.mode === m);
  stage.classList.toggle('pin', m === 'brush');
  stage.classList.toggle('measure', m === 'measure');
  controls.enableRotate = true;
  $('brushbar').hidden = m !== 'brush';
  if (stroke) endStroke();
  showRing(null);
  measure = [];
  clearDraft();
  $('chip').hidden = true;
  if (m === 'brush') chip('<span>Drag on the part to paint · drag the background to rotate</span>');
  if (m === 'measure') chip('<span>Tap the first point</span>');
}

// ── Selectors ────────────────────────────────────────────────────────────────

const opt = (v, t, sel) => `<option value="${esc(v)}"${sel ? ' selected' : ''}>${esc(t)}</option>`;
const ago = (t) => {
  const s = (Date.now() - new Date(t)) / 1000;
  return s < 3600 ? `${Math.round(s / 60)}m` : s < 86400 ? `${Math.round(s / 3600)}h` : `${Math.round(s / 86400)}d`;
};

function renderSelectors() {
  $('project').innerHTML = S.projects.map((p) => opt(p.name, p.name, p === S.project)).join('');
  $('part').innerHTML = S.project.parts.map((pt) =>
    opt(pt.name, `${pt.name}${pt.ref ? ' (ref)' : ''} · ${pt.files.length}`, pt === S.part)).join('');
  const files = S.part.files;
  $('version').innerHTML = files.slice().reverse().map((f) =>
    opt(f.path, `${label(f)} · ${ago(f.mtime)} ago`, f === S.file)).join('');
  const i = files.indexOf(S.file);
  $('prev').disabled = i <= 0;
  $('next').disabled = i >= files.length - 1;
  let html = opt('', 'Compare: off', !S.cmp);
  for (const pt of S.project.parts) {
    html += `<optgroup label="${esc(pt.name)}">` + pt.files.slice().reverse()
      .filter((f) => f !== S.file).map((f) => opt(f.path, `vs ${pt === S.part ? "" : pt.name + " "}${label(f)}`, f === S.cmp)).join('') + '</optgroup>';
  }
  $('compare').innerHTML = html;
  for (const b of $('cmpmode').children) b.classList.toggle('on', b.dataset.m === S.cmpMode);
  $('cmpmode').classList.toggle('disabled', !S.cmp);
}

const findFile = (path) => S.project.parts.flatMap((pt) => pt.files).find((f) => f.path === path) || null;
const partOf = (f) => S.project.parts.find((pt) => pt.files.includes(f));
const prevVersion = (f) => { const fs = partOf(f).files; const i = fs.indexOf(f); return i > 0 ? fs[i - 1] : null; };

function selectFile(f, { cmp = 'auto', fit = false } = {}) {
  S.file = f;
  S.part = partOf(f);
  S.flipped = false;
  if (cmp === 'auto') S.cmp = S.cmp && S.cmp !== f && findFile(S.cmp.path) ? S.cmp : null;
  else S.cmp = cmp;
  writeHash();
  renderSelectors();
  loadView({ fit });
}

function selectProject(p) {
  S.project = p;
  S.part = p.parts[0];
  const f = S.part.files[S.part.files.length - 1];
  fetchAnnotations();
  selectFile(f, { cmp: prevVersion(f), fit: true });
}

$('project').onchange = (e) => selectProject(S.projects.find((p) => p.name === e.target.value));
$('part').onchange = (e) => {
  const pt = S.project.parts.find((x) => x.name === e.target.value);
  const f = pt.files[pt.files.length - 1];
  selectFile(f, { cmp: prevVersion(f), fit: true });
};
$('version').onchange = (e) => { const f = findFile(e.target.value); selectFile(f, { cmp: S.cmp === f ? prevVersion(f) : 'auto' }); };
$('prev').onclick = () => { const fs = S.part.files, i = fs.indexOf(S.file); if (i > 0) selectFile(fs[i - 1], { cmp: i > 1 ? fs[i - 2] : null }); };
$('next').onclick = () => { const fs = S.part.files, i = fs.indexOf(S.file); if (i < fs.length - 1) selectFile(fs[i + 1], { cmp: fs[i] }); };
$('compare').onchange = (e) => { S.cmp = findFile(e.target.value); S.flipped = false; writeHash(); renderSelectors(); loadView(); };
$('cmpmode').onclick = (e) => {
  const m = e.target.dataset.m;
  if (!m) return;
  S.cmpMode = m; S.flipped = false;
  try { localStorage.setItem('stlr.cmpMode', m); } catch {}
  writeHash(); renderSelectors(); applyDisplay();
};
$('flipbtn').onclick = () => { S.flipped = !S.flipped; applyDisplay(); };
$('mode').onclick = (e) => { if (e.target.dataset.mode) setMode(e.target.dataset.mode); };
$('views').onclick = (e) => { if (e.target.dataset.v) setView(e.target.dataset.v); };
$('section').onclick = () => {
  S.section.on = !S.section.on;
  $('section').classList.toggle('on', S.section.on);
  $('sectionbar').hidden = !S.section.on;
  syncSectionRange();
  scheduleOcclusion();
};
$('secaxis').onclick = (e) => { if (e.target.dataset.a) { S.section.axis = e.target.dataset.a; syncSectionRange(); } };
$('secpos').oninput = (e) => { S.section.t = e.target.value / 1000; syncSectionRange(); };
$('secflip').onclick = () => { S.section.flip = !S.section.flip; syncSectionRange(); };
$('showresolved').onchange = (e) => { S.showResolved = e.target.checked; renderPins(); };
$('openpanel').onclick = () => document.body.classList.add('panel-open');
$('closepanel').onclick = () => document.body.classList.remove('panel-open');
window.addEventListener('keydown', (e) => {
  if (e.target.matches('input, textarea, select')) return;
  if (e.key === ' ' && cmpGeo) { e.preventDefault(); S.flipped = !S.flipped; applyDisplay(); }
  if (e.key === 'Escape') { setMode('view'); closeSheet(); }
  if (e.key === 'b') setMode('brush');
  if (e.key === 'm') setMode('measure');
  if (e.key === '[') setBrush(brushD - 0.5);
  if (e.key === ']') setBrush(brushD + 0.5);
  if (e.key === 'x' && isBrush()) setErase(!brushErase);
  if (e.key === 'v') setMode('view');
  if (e.key === 'f') setView('fit');
});

function setErase(on) {
  brushErase = on;
  for (const b of $('brushmode').children) b.classList.toggle('on', (b.dataset.b === 'erase') === on);
  ring.material.color.set(on ? 0x3d7fe0 : 0xff7a00);
  requestRender();
}
$('brushmode').onclick = (e) => { if (e.target.dataset.b) setErase(e.target.dataset.b === 'erase'); };
$('brushsize').oninput = (e) => setBrush(parseFloat(e.target.value));
$('paintclear').onclick = clearPaint;
$('paintnote').onclick = () => {
  const region = paintRegion(paint.dabs, mainMesh.geometry);
  if (!region) { toast('Nothing painted yet'); return; }
  pending = { region };
  openSheet(`${label(activeFile())} · paint · ${region.size.map((v) => fmt(v, 1)).join('×')} mm · ~${fmt(region.area_mm2, 0)} mm² around ${fmtPt(region.centroid)}`, '');
};

// ── URL hash: #p=<project>&f=<path>&c=<path>&m=<mode> ─────────────────────────

function writeHash() {
  const h = new URLSearchParams({ p: S.project.name, f: S.file.path });
  if (S.cmp) { h.set('c', S.cmp.path); h.set('m', S.cmpMode); }
  history.replaceState(null, '', '#' + h);
}

function applyHash() {
  const h = new URLSearchParams(location.hash.slice(1));
  const p = S.projects.find((x) => x.name === h.get('p'));
  if (!p) return false;
  S.project = p;
  const f = findFile(h.get('f'));
  if (!f) return false;
  if (h.get('m')) S.cmpMode = h.get('m');
  fetchAnnotations();
  selectFile(f, { cmp: findFile(h.get('c')), fit: true });
  return true;
}

// ── Live updates ─────────────────────────────────────────────────────────────

// When a new version of the part on screen is published, jump to it and diff against the
// one that was showing — the "I just released v15" loop needs no clicks.
async function refreshProjects() {
  const wasLatest = S.part && S.file === S.part.files[S.part.files.length - 1];
  const oldFile = S.file, oldCmp = S.cmp;
  S.projects = await (await fetch('/api/projects')).json();
  if (!S.project) return;
  S.project = S.projects.find((p) => p.name === S.project.name) || S.projects[0];
  if (!S.project) return;
  const same = (f) => f && findFile(f.path);
  const cur = same(oldFile);
  const part = cur ? partOf(cur) : S.project.parts[0];
  const newest = part.files[part.files.length - 1];
  if (wasLatest && cur && newest !== cur && newest.mtime > cur.mtime) {
    toast(`${label(newest)} arrived — diffing against ${label(cur)}`);
    selectFile(newest, { cmp: cur });
  } else if (cur) {
    const changed = cur.mtime !== oldFile.mtime || cur.size !== oldFile.size || (oldCmp && same(oldCmp)?.mtime !== oldCmp.mtime);
    S.file = cur; S.part = part; S.cmp = same(oldCmp);
    renderSelectors();
    if (changed) loadView();
  } else {
    selectProject(S.project);
  }
}

async function poll() {
  try {
    const st = await (await fetch('/api/state')).json();
    if (S.stamp.meshes && st.meshes !== S.stamp.meshes) await refreshProjects();
    if (S.stamp.annotations && st.annotations !== S.stamp.annotations) await fetchAnnotations();
    S.stamp = st;
  } catch {}
  setTimeout(poll, document.hidden ? 15000 : 3000);
}

// ── Boot ─────────────────────────────────────────────────────────────────────

async function boot() {
  try { S.cmpMode = localStorage.getItem('stlr.cmpMode') || S.cmpMode; } catch {}
  setMode('view');
  $('chip').hidden = true;
  S.projects = await (await fetch('/api/projects')).json();
  if (!S.projects.length) { busy('No projects found (looking for ~/*/stl/*.stl)'); return; }
  if (!applyHash()) selectProject(S.projects[0]);
  resize();
  loop();
  poll();
}
boot();
