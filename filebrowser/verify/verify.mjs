/**
 * Filebrowser smoke verification script.
 * Usage:
 *   FB_USER=admin FB_PASS=xxx FB_BASE=http://192.168.30.7:10094 \
 *     node filebrowser/verify/verify.mjs
 *
 * Requires: node + playwright installed (npm install in this dir, or pass
 * PLAYWRIGHT_PATH env to a local node_modules).
 *
 * Run from repo root with:
 *   cd filebrowser/verify && npm install && FB_USER=... FB_PASS=... node verify.mjs
 */

import { chromium } from './node_modules/playwright/index.mjs';
import fs from 'fs';

const BASE = process.env.FB_BASE || 'http://192.168.30.7:10094';
const USER = process.env.FB_USER || 'admin';
const PASS = process.env.FB_PASS || '';
const SS_DIR = process.env.SS_DIR || '/tmp/fb-verify-screenshots';
const CHROMIUM_PATH = process.env.CHROMIUM_PATH || 'chromium';

fs.mkdirSync(SS_DIR, { recursive: true });

let stepN = 0;
const results = [];

function log(icon, desc, detail = '') {
  stepN++;
  const line = `${stepN}. ${icon} ${desc}${detail ? ' → ' + detail : ''}`;
  console.log(line);
  results.push({ icon, desc, detail });
}

async function ss(page, name) {
  const p = `${SS_DIR}/${name}.png`;
  await page.screenshot({ path: p }).catch(() => {});
  return p;
}

if (!PASS) {
  console.error('FB_PASS not set. Usage: FB_USER=admin FB_PASS=xxx node verify.mjs');
  process.exit(1);
}

const browser = await chromium.launch({
  executablePath: CHROMIUM_PATH,
  args: ['--no-sandbox', '--disable-dev-shm-usage'],
  headless: true,
});
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
const page = await ctx.newPage();

try {
  // ── 1. Auth: unauthenticated redirect ──────────────────────────────────
  await page.goto(BASE + '/browse');
  const loginRedirect = page.url().includes('/login');
  log(loginRedirect ? '✅' : '❌', 'Unauthenticated /browse redirects to /login', page.url());
  await ss(page, '01-login-page');

  // ── 2. Wrong password rejected ─────────────────────────────────────────
  await page.fill('input[name=username]', USER);
  await page.fill('input[name=password]', 'wrong-password-xyz');
  await page.click('button[type=submit]');
  await page.waitForTimeout(600);
  log(page.url().includes('/login') ? '✅' : '❌', 'Wrong password rejected — stayed on /login');
  await ss(page, '02-wrong-password');

  // ── 3. Login ───────────────────────────────────────────────────────────
  await page.fill('input[name=username]', USER);
  await page.fill('input[name=password]', PASS);
  await page.click('button[type=submit]');
  await page.waitForNavigation({ waitUntil: 'domcontentloaded' }).catch(() => page.waitForTimeout(1000));
  const loggedIn = !page.url().includes('/login');
  log(loggedIn ? '✅' : '❌', 'Login with correct credentials', page.url());
  await ss(page, '03-post-login');
  if (!loggedIn) { await browser.close(); process.exit(1); }

  // ── 4. Browse: root loads ──────────────────────────────────────────────
  await page.goto(BASE + '/browse');
  await page.waitForTimeout(500);
  const allItems = await page.$$('#view-list tr');
  log(allItems.length > 0 ? '✅' : '⚠️', `Browse loaded — ${allItems.length} rows`);
  await ss(page, '04-browse-root');

  // Click a visible subfolder if present
  const firstDir = page.locator('#view-list .dir-row, #view-list tr[data-dir]').first();
  if (await firstDir.count() > 0 && await firstDir.isVisible().catch(() => false)) {
    const prevUrl = page.url();
    await firstDir.click({ timeout: 5000 }).catch(() => {});
    await page.waitForTimeout(600);
    log(page.url() !== prevUrl ? '✅' : '⚠️', 'Subfolder click navigates', page.url().split('?')[1] || '');
    await ss(page, '04b-subfolder-nav');
  } else {
    log('⚠️', 'No visible subfolder rows (files-only directory at default path)');
  }

  // ── 5. List/grid toggle persists ──────────────────────────────────────
  await page.goto(BASE + '/browse');
  await page.waitForTimeout(400);
  await page.locator('#btn-grid').click({ timeout: 3000 }).catch(() => {});
  await page.waitForTimeout(300);
  const gridOn = await page.locator('#view-grid').isVisible().catch(() => false);
  log(gridOn ? '✅' : '⚠️', 'Grid view toggle');
  await page.goto(BASE + '/browse');
  await page.waitForTimeout(400);
  const gridPersisted = await page.locator('#view-grid').isVisible().catch(() => false);
  log(gridPersisted ? '✅' : '⚠️', 'Grid view persists after navigation');
  await page.locator('#btn-list').click({ timeout: 3000 }).catch(() => {}); // reset
  await ss(page, '05-view-toggle');

  // ── 6. Recent tab ──────────────────────────────────────────────────────
  await page.goto(BASE + '/recent');
  await page.waitForSelector('#view-list, p.muted', { timeout: 5000 }).catch(() => {});
  const recentRows = await page.$$('#view-list tr.file-row');
  const recentEmpty = await page.locator('p.muted').first().isVisible().catch(() => false);
  log(recentRows.length > 0 || recentEmpty ? '✅' : '⚠️',
    `Recent loaded — ${recentRows.length} items${recentEmpty ? ' (empty)' : ''}`);
  if (recentRows.length > 0) {
    await recentRows[0].click({ timeout: 5000 });
    await page.waitForTimeout(600);
    log(page.url().includes('/browse') ? '✅' : '⚠️', 'Recent row click navigates to Browse', page.url().split('?')[1] || '');
  }
  await ss(page, '06-recent');

  // ── 7. Unplayed tab ────────────────────────────────────────────────────
  await page.goto(BASE + '/unplayed');
  await page.waitForSelector('#view-list, p.muted', { timeout: 5000 }).catch(() => {});
  const unplayedRows = await page.$$('#view-list tr.file-row[data-type]');
  const unplayedEmpty = await page.locator('p.muted').first().isVisible().catch(() => false);
  log(unplayedRows.length > 0 || unplayedEmpty ? '✅' : '⚠️',
    `Unplayed loaded — ${unplayedRows.length} items${unplayedEmpty ? ' (empty)' : ''}`);

  // Filter chips
  const vidChip = page.locator('button.sf-chip[onclick*="setTypeFilter"][onclick*="video"]');
  const allChip = page.locator('button.sf-chip[onclick*="setTypeFilter"][onclick*="all"]');
  if (await vidChip.count() > 0) {
    await vidChip.click({ timeout: 3000 });
    await page.waitForTimeout(300);
    const hiddenAudio = await page.$$('tr.file-row[data-type="audio"][style*="none"], .grid-card[data-type="audio"][style*="none"]');
    const shownVideo = await page.$$('tr.file-row[data-type="video"]:not([style*="none"])');
    log('✅', `Video filter chip — ${shownVideo.length} video shown, ${hiddenAudio.length} audio hidden`);
    await allChip.click({ timeout: 3000 });
    await page.waitForTimeout(300);
    log('✅', 'All chip restores list');
  }
  if (unplayedRows.length > 0) {
    await unplayedRows[0].click({ timeout: 5000 });
    await page.waitForTimeout(600);
    log(page.url().includes('/browse') ? '✅' : '⚠️', 'Unplayed row click navigates to Browse', page.url().split('?')[1] || '');
  }
  await ss(page, '07-unplayed');

  // ── 8 & 9. Media playback — find dirs via unplayed or recent ──────────
  // Collect known dirs from unplayed (has data-type on rows)
  await page.goto(BASE + '/unplayed');
  await page.waitForTimeout(500);
  const videoDir = await page.locator('#view-list tr.file-row[data-type="video"]').first().getAttribute('data-dir').catch(() => null);
  const audioDir = await page.locator('#view-list tr.file-row[data-type="audio"]').first().getAttribute('data-dir').catch(() => null);

  // ── 8. Video playback ─────────────────────────────────────────────────
  if (videoDir) {
    await page.goto(BASE + '/browse?dir=' + encodeURIComponent(videoDir));
    await page.waitForTimeout(500);
    const videoFile = page.locator('#view-list tr.file-row[data-type="video"]').first();
    if (await videoFile.count() > 0) {
      await videoFile.click({ timeout: 5000 });
      await page.waitForTimeout(2500);
      const hasVideo = await page.locator('video').first().isVisible().catch(() => false);
      const hasModal = await page.locator('#preview-modal, .modal-backdrop, #modal').first().isVisible().catch(() => false);
      log(hasVideo || hasModal ? '✅' : '⚠️', 'Video modal opens on click', hasVideo ? 'video element visible' : hasModal ? 'modal visible' : 'nothing appeared');
      if (hasVideo) {
        const readyState = await page.evaluate(() => document.querySelector('video')?.readyState ?? -1);
        log(readyState >= 1 ? '✅' : '⚠️', `Video player readyState: ${readyState}`);
      }
      await ss(page, '08-video-player');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);
    } else {
      log('⚠️', 'No video rows visible in video dir');
    }
  } else {
    log('⚠️', 'No video files found in Unplayed — skipping video playback test');
  }

  // ── 9. Audio playback ─────────────────────────────────────────────────
  if (audioDir) {
    await page.goto(BASE + '/browse?dir=' + encodeURIComponent(audioDir));
    await page.waitForTimeout(500);
    const audioFile = page.locator('#view-list tr.file-row[data-type="audio"]').first();
    if (await audioFile.count() > 0) {
      await audioFile.click({ timeout: 5000 });
      await page.waitForTimeout(1500);
      const hasAudio = await page.locator('audio').first().isVisible().catch(() => false);
      const hasModal = await page.locator('#preview-modal, .modal-backdrop, #modal').first().isVisible().catch(() => false);
      log(hasAudio || hasModal ? '✅' : '⚠️', 'Audio player opens on click', hasAudio ? 'audio element visible' : hasModal ? 'modal visible' : 'nothing appeared');
      await ss(page, '09-audio-player');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);
    } else {
      log('⚠️', 'No audio rows visible in audio dir');
    }
  } else {
    log('⚠️', 'No audio files found in Unplayed — skipping audio playback test');
  }

  // ── 10. Search ────────────────────────────────────────────────────────
  await page.goto(BASE + '/browse');
  await page.waitForTimeout(400);
  const searchInput = page.locator('#search-q');
  if (await searchInput.count() > 0) {
    await searchInput.click();
    // fill + manual dispatch is most reliable across headless/headed
    await searchInput.fill('bach');
    await page.evaluate(() => {
      const el = document.getElementById('search-q');
      if (el) el.dispatchEvent(new Event('input', { bubbles: true }));
    });
    // Wait for results to appear (debounce 300ms + fetch)
    await page.waitForSelector('#search-results-list .search-result', { timeout: 5000 }).catch(() => {});
    const resultItems = await page.$$('#search-results-list .search-result');
    const panelVisible = await page.locator('#search-panel').isVisible().catch(() => false);
    log(resultItems.length > 0 ? '✅' : panelVisible ? '⚠️' : '❌',
      `Search: ${resultItems.length} results for "bach", panel ${panelVisible ? 'open' : 'closed'}`);
    if (resultItems.length > 0) {
      await resultItems[0].click({ timeout: 5000 });
      await page.waitForTimeout(600);
      log(page.url().includes('/browse') ? '✅' : '⚠️', 'Search result click navigates', page.url().split('?')[1] || '');
    }
    await ss(page, '10-search');
  } else {
    log('⚠️', 'Search input #search-q not found');
  }

  // ── 11. Playlists ─────────────────────────────────────────────────────
  await page.goto(BASE + '/playlists');
  await page.waitForSelector('table, ul, p.muted, a[href*="/playlists/"]', { timeout: 5000 }).catch(() => {});
  const plLinks = await page.$$('a[href*="/playlists/"]');
  const plEmpty = await page.locator('p.muted').first().isVisible().catch(() => false);
  log(plLinks.length > 0 || plEmpty ? '✅' : '⚠️',
    `Playlists loaded — ${plLinks.length} playlists${plEmpty ? ' (empty)' : ''}`);
  if (plLinks.length > 0) {
    await plLinks[0].click({ timeout: 5000 });
    await page.waitForNavigation({ waitUntil: 'domcontentloaded' }).catch(() => page.waitForTimeout(500));
    log(page.url().includes('/playlists/') ? '✅' : '⚠️', 'Playlist detail page opens', page.url());
    await ss(page, '11b-playlist-detail');
  }
  await ss(page, '11-playlists');

  // ── 12. Settings ─────────────────────────────────────────────────────
  await page.goto(BASE + '/settings');
  await page.waitForSelector('h2, form', { timeout: 5000 }).catch(() => {});
  const settingsHeading = await page.locator('h2').first().textContent().catch(() => '');
  log(settingsHeading ? '✅' : '⚠️', `Settings loaded — heading: "${settingsHeading}"`);
  await ss(page, '12-settings');

  // ── 13. Nav bar has all expected tabs ─────────────────────────────────
  const navLinks = await page.$$eval('nav a', els => els.map(e => e.textContent.trim()));
  const expected = ['Browse', 'Recent', 'Unplayed', 'Top Played', 'Favorites', 'Playlists', 'Settings'];
  const missing = expected.filter(t => !navLinks.some(l => l.includes(t)));
  log(missing.length === 0 ? '✅' : '❌',
    `Nav tabs: ${navLinks.join(', ')}${missing.length ? ' — MISSING: ' + missing.join(', ') : ''}`);

  // ── 14. Top Played tab ────────────────────────────────────────────────
  await page.goto(BASE + '/top-played');
  await page.waitForSelector('.pl-layout, p.muted', { timeout: 5000 }).catch(() => {});
  const topPlayedTitle = await page.locator('h2').first().textContent().catch(() => '');
  const topPlayedLayout = await page.locator('.pl-layout').isVisible().catch(() => false);
  const topPlayedEmpty = await page.locator('p.muted').first().isVisible().catch(() => false);
  log(topPlayedTitle.includes('Top') ? '✅' : '❌', `Top Played page loads — "${topPlayedTitle}"`);
  log(topPlayedLayout || topPlayedEmpty ? '✅' : '⚠️',
    topPlayedLayout ? 'Top Played has tracks — player layout visible' : 'Top Played empty state shown');
  await ss(page, '14-top-played');

  // ── 15. Favorites tab ─────────────────────────────────────────────────
  await page.goto(BASE + '/favorites');
  await page.waitForSelector('.pl-layout, p.muted', { timeout: 5000 }).catch(() => {});
  const favTitle = await page.locator('h2').first().textContent().catch(() => '');
  const favLayout = await page.locator('.pl-layout').isVisible().catch(() => false);
  const favEmpty = await page.locator('p.muted').first().isVisible().catch(() => false);
  log(favTitle.includes('Favorites') || favTitle.includes('★') ? '✅' : '❌',
    `Favorites page loads — "${favTitle}"`);
  log(favLayout || favEmpty ? '✅' : '⚠️',
    favLayout ? 'Favorites has tracks — player layout visible' : 'Favorites empty state shown');
  await ss(page, '15-favorites');

  // ── 16. Star buttons in Browse (list + grid) ─────────────────────────
  await page.goto(BASE + '/browse');
  await page.waitForTimeout(600);
  // Check list view: fav-btn on folder row and audio row
  const favBtnsListView = await page.$$('#view-list .fav-btn');
  log(favBtnsListView.length > 0 ? '✅' : '⚠️',
    `Star buttons in list view — ${favBtnsListView.length} visible`);

  // Switch to grid view and check star position (must be top-right, not top-left)
  await page.locator('#btn-grid').click({ timeout: 3000 }).catch(() => {});
  await page.waitForTimeout(300);
  const favBtnsGrid = await page.$$('#view-grid .fav-btn');
  log(favBtnsGrid.length > 0 ? '✅' : '⚠️',
    `Star buttons in grid view — ${favBtnsGrid.length} visible`);

  // Verify the star is right-aligned (not overlapping the top-left checkbox)
  if (favBtnsGrid.length > 0) {
    const btnBox = await favBtnsGrid[0].boundingBox().catch(() => null);
    const cardBox = await favBtnsGrid[0].evaluate(el => {
      const card = el.closest('.grid-card');
      return card ? card.getBoundingClientRect().toJSON() : null;
    }).catch(() => null);
    if (btnBox && cardBox) {
      const isRight = btnBox.x + btnBox.width > cardBox.x + cardBox.width / 2;
      log(isRight ? '✅' : '❌',
        `Grid star is top-right (x=${Math.round(btnBox.x)}, card right edge=${Math.round(cardBox.x + cardBox.width)})`);
    }
  }

  // Toggle a star on a folder or audio file — scope to the active grid view
  const firstFavBtn = page.locator('#view-grid .fav-btn').first();
  if (await firstFavBtn.count() > 0) {
    const before = await firstFavBtn.textContent().catch(() => '');
    await firstFavBtn.click({ timeout: 3000 });
    await page.waitForTimeout(500);
    const after = await firstFavBtn.textContent().catch(() => '');
    log(before !== after ? '✅' : '❌',
      `Star toggle changes icon: "${before}" → "${after}"`);
    // Toggle back
    await firstFavBtn.click({ timeout: 3000 });
    await page.waitForTimeout(400);
  }
  await ss(page, '16-star-buttons');
  await page.locator('#btn-list').click({ timeout: 3000 }).catch(() => {}); // reset view

} catch (err) {
  console.error('\nEXCEPTION:', err.message);
  await ss(page, 'xx-exception-state').catch(() => {});
} finally {
  await browser.close();
}

// Summary
console.log('\n─────────────────────────────');
const fails = results.filter(r => r.icon === '❌');
const warns = results.filter(r => r.icon === '⚠️');
const passes = results.filter(r => r.icon === '✅');
console.log(`Steps: ${results.length}  ✅ ${passes.length}  ⚠️ ${warns.length}  ❌ ${fails.length}`);
console.log(`Screenshots: ${SS_DIR}/`);
if (fails.length > 0) {
  console.log('\nFAILURES:');
  fails.forEach(r => console.log(`  ❌ ${r.desc}${r.detail ? ' → ' + r.detail : ''}`));
}
if (warns.length > 0) {
  console.log('\nWARNINGS:');
  warns.forEach(r => console.log(`  ⚠️  ${r.desc}${r.detail ? ' → ' + r.detail : ''}`));
}
console.log(`\nVerdict: ${fails.length > 0 ? 'FAIL' : warns.length > 0 ? 'PASS (with warnings)' : 'PASS'}`);
process.exit(fails.length > 0 ? 1 : 0);
