#!/usr/bin/env node
/**
 * Reproducible Documentation Media Renderer for sshm (Oktanetflow)
 *
 * Generates:
 *  - 6 High-DPI TUI Mockups (.png) in docs/assets/mockups/
 *  - 3 Illustrated How-To Demo Videos (.gif + .webm) in docs/assets/videos/
 *
 * Requirements: Node.js 18+ and Microsoft Edge or Google Chrome (zero npm/ffmpeg installs).
 * Usage: node docs/scripts/render-media.mjs
 */

import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import { spawnSync, spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, "..", "..");
const ILLUST_DIR = path.join(REPO_ROOT, "docs", "assets", "illustrations");
const MOCKUP_DIR = path.join(REPO_ROOT, "docs", "assets", "mockups");
const VIDEO_DIR = path.join(REPO_ROOT, "docs", "assets", "videos");

fs.mkdirSync(MOCKUP_DIR, { recursive: true });
fs.mkdirSync(VIDEO_DIR, { recursive: true });

function findBrowser() {
  const candidates = [
    process.env.CHROME_PATH,
    "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
    "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
    "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
    "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  ].filter(Boolean);
  for (const p of candidates) {
    if (fs.existsSync(p)) return p;
  }
  throw new Error("Could not find Chrome or Edge browser executable. Set CHROME_PATH.");
}

// ---------------------------------------------------------------------------
// Pure-JS Spec-Compliant GIF89a Encoder (256-color Palette + LZW)
// ---------------------------------------------------------------------------
function buildPalette() {
  // 216 web-safe 6x6x6 cube + 40 curated dark/light terminal grays & accent colors
  const palette = [];
  const levels = [0, 51, 102, 153, 204, 255];
  for (const r of levels) {
    for (const g of levels) {
      for (const b of levels) {
        palette.push([r, g, b]);
      }
    }
  }
  const extras = [
    [13, 17, 23],   // #0D1117 bg
    [14, 17, 23],   // #0E1117 term
    [22, 27, 34],   // #161B22 card
    [30, 38, 48],   // #1E2630 border
    [38, 48, 64],   // #263040 divider
    [68, 68, 68],   // #444444
    [95, 215, 255], // #5FD7FF xterm 81 cyan
    [135, 175, 215],// #87AFD7 xterm 110
    [135, 215, 135],// #87D787 xterm 114 green
    [135, 215, 255],// #87D7FF xterm 117
    [175, 135, 255],// #AF87FF xterm 141 purple
    [215, 135, 255],// #D787FF xterm 177
    [255, 95, 95],  // #FF5F5F xterm 203 red
    [255, 175, 95], // #FFAF5F xterm 215 amber
    [255, 215, 0],  // #FFD700 xterm 220 gold
    [255, 215, 95], // #FFD75F xterm 221 yellow
    [208, 208, 208],// #D0D0D0 xterm 252
    [128, 128, 128],// #808080 xterm 244
    [250, 251, 253],// #FAFBFD light bg
    [241, 245, 249],// #F1F5F9 light panel
    [0, 95, 135],   // #005F87 xterm 24 light accent
    [0, 135, 0],    // #008700 xterm 28 light green
    [215, 0, 0],    // #D70000 xterm 160 light red
    [175, 95, 0],   // #AF5F00 xterm 130 light amber
    [38, 38, 38],   // #262626 xterm 235
    [118, 118, 118],// #767676 xterm 243
    [188, 188, 188],// #BCBCBC xterm 250
    [201, 209, 217],// #C9D1D9 github fg
    [139, 148, 158],// #8B949E github muted
    [17, 34, 51],   // #112233 badge bg
    [20, 44, 31],   // #142C1F green badge bg
    [35, 25, 54],   // #231936 purple badge bg
    [51, 35, 18],   // #332312 amber badge bg
    [56, 189, 248], // #38BDF8 sky
    [240, 246, 252],// #F0F6FC bright white
    [17, 17, 17],   // #111111 mono bg
    [230, 230, 230],// #E6E6E6 mono fg
    [122, 122, 122],// #7A7A7A mono dim
    [95, 175, 255], // #5FAFFF blue dot
    [25, 32, 43],   // #19202B header
  ];
  for (const c of extras) palette.push(c);
  return palette; // exact length 256
}

const GLOBAL_PALETTE = buildPalette();

// Fast 15-bit RGB (5-5-5) lookup table -> palette index
const COLOR_LUT = new Uint8Array(32768);
(function initLut() {
  for (let r5 = 0; r5 < 32; r5++) {
    const r = (r5 << 3) | (r5 >> 2);
    for (let g5 = 0; g5 < 32; g5++) {
      const g = (g5 << 3) | (g5 >> 2);
      for (let b5 = 0; b5 < 32; b5++) {
        const b = (b5 << 3) | (b5 >> 2);
        let bestIdx = 0;
        let bestDist = Infinity;
        for (let i = 0; i < 256; i++) {
          const pr = GLOBAL_PALETTE[i][0];
          const pg = GLOBAL_PALETTE[i][1];
          const pb = GLOBAL_PALETTE[i][2];
          const dr = r - pr;
          const dg = g - pg;
          const db = b - pb;
          const d = dr * dr + dg * dg + db * db;
          if (d < bestDist) {
            bestDist = d;
            bestIdx = i;
            if (d === 0) break;
          }
        }
        COLOR_LUT[(r5 << 10) | (g5 << 5) | b5] = bestIdx;
      }
    }
  }
})();

function quantizeRGBA(rgbaBuf, pixelCount) {
  const indexed = new Uint8Array(pixelCount);
  for (let i = 0, p = 0; i < pixelCount; i++, p += 4) {
    const r5 = rgbaBuf[p] >> 3;
    const g5 = rgbaBuf[p + 1] >> 3;
    const b5 = rgbaBuf[p + 2] >> 3;
    indexed[i] = COLOR_LUT[(r5 << 10) | (g5 << 5) | b5];
  }
  return indexed;
}

function lzwEncode(indexed, minCodeSize = 8) {
  const clearCode = 1 << minCodeSize;
  const eoiCode = clearCode + 1;
  const out = [];
  let bitBuf = 0;
  let bitCount = 0;

  const emitCode = (code, width) => {
    bitBuf |= code << bitCount;
    bitCount += width;
    while (bitCount >= 8) {
      out.push(bitBuf & 0xff);
      bitBuf >>= 8;
      bitCount -= 8;
    }
  };

  let codeSize = minCodeSize + 1;
  let nextCode = eoiCode + 1;
  // Trie table: flat Int32Array(4096 * 256) is 4MB; Map with numeric key is super clean and fast
  let dict = new Map();

  emitCode(clearCode, codeSize);
  let prefix = indexed[0];

  for (let i = 1; i < indexed.length; i++) {
    const k = indexed[i];
    const key = (prefix << 8) | k;
    const found = dict.get(key);
    if (found !== undefined) {
      prefix = found;
    } else {
      emitCode(prefix, codeSize);
      if (nextCode < 4096) {
        dict.set(key, nextCode);
        if (nextCode === (1 << codeSize) && codeSize < 12) {
          codeSize++;
        }
        nextCode++;
      } else {
        emitCode(clearCode, codeSize);
        dict.clear();
        codeSize = minCodeSize + 1;
        nextCode = eoiCode + 1;
      }
      prefix = k;
    }
  }
  emitCode(prefix, codeSize);
  emitCode(eoiCode, codeSize);
  if (bitCount > 0) {
    out.push(bitBuf & 0xff);
  }
  return out;
}

function encodeGIF(width, height, frames) {
  const chunks = [];
  const pushBytes = (arr) => chunks.push(Buffer.from(arr));
  const pushU16 = (v) => pushBytes([v & 0xff, (v >> 8) & 0xff]);

  // Header
  chunks.push(Buffer.from("GIF89a", "ascii"));
  // Logical Screen Descriptor
  pushU16(width);
  pushU16(height);
  pushBytes([0xf7, 0x00, 0x00]); // Global Color Table flag=1, colorRes=7, sort=0, size=7 (256 colors)
  // Global Color Table (256 * 3 bytes)
  const gct = Buffer.alloc(256 * 3);
  for (let i = 0; i < 256; i++) {
    gct[i * 3] = GLOBAL_PALETTE[i][0];
    gct[i * 3 + 1] = GLOBAL_PALETTE[i][1];
    gct[i * 3 + 2] = GLOBAL_PALETTE[i][2];
  }
  chunks.push(gct);

  // Netscape 2.0 Application Extension (infinite loop)
  pushBytes([0x21, 0xff, 0x0b]);
  chunks.push(Buffer.from("NETSCAPE2.0", "ascii"));
  pushBytes([0x03, 0x01, 0x00, 0x00, 0x00]);

  for (const frame of frames) {
    // Graphic Control Extension
    const delayCs = Math.max(2, Math.round(frame.delayMs / 10));
    pushBytes([0x21, 0xf9, 0x04, 0x04, delayCs & 0xff, (delayCs >> 8) & 0xff, 0x00, 0x00]);

    // Image Descriptor
    pushBytes([0x2c]);
    pushU16(0);
    pushU16(0);
    pushU16(width);
    pushU16(height);
    pushBytes([0x00]); // No local color table, not interlaced

    // Image Data
    const minCodeSize = 8;
    pushBytes([minCodeSize]);
    const lzw = lzwEncode(frame.indexed, minCodeSize);
    for (let pos = 0; pos < lzw.length; pos += 255) {
      const slice = lzw.slice(pos, pos + 255);
      pushBytes([slice.length]);
      chunks.push(Buffer.from(slice));
    }
    pushBytes([0x00]); // Block terminator
  }

  // Trailer
  pushBytes([0x3b]);
  return Buffer.concat(chunks);
}

// ---------------------------------------------------------------------------
// High-DPI TUI Mockup HTML Generator (Exact styles.go Palette)
// ---------------------------------------------------------------------------
const THEMES = {
  "minimal-dark": {
    bg: "#0E1117",
    chromeBg: "#161B22",
    chromeBorder: "#263040",
    title: "#5FD7FF",
    stats: "#87AFD7",
    meta: "#808080",
    divider: "#444444",
    text: "#D0D0D0",
    dim: "#6C6C6C",
    tabActiveBg: "#5FD7FF",
    tabActiveFg: "#0E1117",
    selBg: "#5FD7FF",
    selFg: "#0E1117",
    pw: "#FFAF5F",
    key: "#87D787",
    jump: "#AF87FF",
    tag: "#87D7FF",
    fwd: "#D787FF",
    online: "#87D787",
    offline: "#FF5F5F",
    checking: "#FFD75F",
    pin: "#FFD700",
    dotRed: "#FF5F5F",
    dotYellow: "#FFD75F",
    dotCyan: "#5FD7FF",
    formBorder: "#5FD7FF",
    formBg: "#121720",
    mono: false,
  },
  "minimal-light": {
    bg: "#FAFBFD",
    chromeBg: "#EAEFF5",
    chromeBorder: "#CBD5E1",
    title: "#005F87",
    stats: "#4E6E8E",
    meta: "#767676",
    divider: "#BCBCBC",
    text: "#262626",
    dim: "#8A8A8A",
    tabActiveBg: "#005F87",
    tabActiveFg: "#FFFFFF",
    selBg: "#005F87",
    selFg: "#FFFFFF",
    pw: "#AF5F00",
    key: "#008700",
    jump: "#5F5FAF",
    tag: "#005FAF",
    fwd: "#8700AF",
    online: "#008700",
    offline: "#D70000",
    checking: "#AF8700",
    pin: "#D78700",
    dotRed: "#D70000",
    dotYellow: "#AF8700",
    dotCyan: "#005F87",
    formBorder: "#005F87",
    formBg: "#FFFFFF",
    mono: false,
  },
  mono: {
    bg: "#111111",
    chromeBg: "#1A1A1A",
    chromeBorder: "#333333",
    title: "#F0F0F0",
    stats: "#A0A0A0",
    meta: "#7A7A7A",
    divider: "#4A4A4A",
    text: "#E6E6E6",
    dim: "#7A7A7A",
    tabActiveBg: "#E6E6E6",
    tabActiveFg: "#111111",
    selBg: "#E6E6E6",
    selFg: "#111111",
    pw: "#A8A8A8",
    key: "#A8A8A8",
    jump: "#A8A8A8",
    tag: "#A8A8A8",
    fwd: "#A8A8A8",
    online: "#FFFFFF",
    offline: "#7A7A7A",
    checking: "#9A9A9A",
    pin: "#FFFFFF",
    dotRed: "#E6E6E6",
    dotYellow: "#E6E6E6",
    dotCyan: "#E6E6E6",
    formBorder: "#E6E6E6",
    formBg: "#161616",
    mono: true,
  },
};

function renderWindowShell(titleBar, themeName, innerHtml, subtitlePill = "") {
  const t = THEMES[themeName];
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8"/>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    width: 1200px;
    height: 700px;
    background: ${themeName === "minimal-light" ? "linear-gradient(135deg,#DCE6F2,#EEF4FA)" : "linear-gradient(135deg,#090D13,#121A26)"};
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: 'Cascadia Mono', 'Cascadia Code', Consolas, 'Courier New', monospace;
    -webkit-font-smoothing: antialiased;
  }
  .win {
    width: 1136px;
    height: 636px;
    background: ${t.bg};
    border: 1.5px solid ${t.chromeBorder};
    border-radius: 12px;
    box-shadow: 0 24px 60px rgba(0,0,0,0.55);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .titlebar {
    height: 40px;
    background: ${t.chromeBg};
    border-bottom: 1px solid ${t.chromeBorder};
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    font-family: 'Segoe UI', sans-serif;
    font-size: 13px;
    color: ${t.text};
  }
  .tab-pill {
    display: flex;
    align-items: center;
    gap: 8px;
    background: ${t.bg};
    border: 1px solid ${t.chromeBorder};
    border-bottom: none;
    padding: 6px 14px;
    border-radius: 7px 7px 0 0;
    margin-top: 8px;
    font-weight: 600;
    font-size: 12.5px;
  }
  .badge-pill {
    font-size: 11.5px;
    padding: 3px 10px;
    border-radius: 999px;
    border: 1px solid ${t.chromeBorder};
    color: ${t.title};
    font-weight: 600;
  }
  .win-controls { display: flex; gap: 16px; color: ${t.meta}; font-size: 12px; }
  .term {
    flex: 1;
    padding: 18px 22px 14px 22px;
    color: ${t.text};
    font-size: 14.2px;
    line-height: 1.48;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }
  .row-between { display: flex; justify-content: space-between; align-items: center; }
  .divider { border-top: 1px solid ${t.divider}; margin: 9px 0; }
  .title { color: ${t.title}; font-weight: 700; }
  .stats { color: ${t.stats}; }
  .meta { color: ${t.meta}; }
  .dim { color: ${t.dim}; }
  .tab-act { background: ${t.tabActiveBg}; color: ${t.tabActiveFg}; font-weight: 700; padding: 1px 8px; border-radius: 3px; }
  .tab-inact { color: ${t.meta}; padding: 1px 8px; }
  .split { display: flex; flex: 1; gap: 0; margin-top: 6px; overflow: hidden; }
  .left-pane { width: 57%; padding-right: 16px; }
  .right-pane {
    width: 43%;
    border-left: 1px solid ${t.divider};
    padding-left: 20px;
    display: flex;
    flex-direction: column;
    justify-content: flex-start;
  }
  .grp-hdr { font-weight: 700; color: ${t.text}; margin-top: 6px; margin-bottom: 2px; }
  .host-row {
    display: block;
    padding: 2px 6px;
    border-radius: 3px;
    white-space: pre;
  }
  .host-row.sel {
    background: ${t.selBg};
    color: ${t.selFg};
    font-weight: 700;
  }
  .host-row.sel span { color: ${t.selFg} !important; }
  .kv { display: flex; margin: 2px 0; }
  .kv-k { width: 102px; color: ${t.meta}; flex-shrink: 0; }
  .kv-v { color: ${t.text}; }
  .pw { color: ${t.pw}; }
  .key { color: ${t.key}; }
  .jump { color: ${t.jump}; }
  .tag { color: ${t.tag}; }
  .fwd { color: ${t.fwd}; }
  .online { color: ${t.online}; }
  .offline { color: ${t.offline}; }
  .pin { color: ${t.pin}; }
  .watermark { color: ${t.meta}; font-style: italic; font-size: 13px; }
  .modal-wrap {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .modal-box {
    border: 1.8px solid ${t.formBorder};
    background: ${t.formBg};
    border-radius: 10px;
    padding: 22px 28px;
    min-width: 720px;
    box-shadow: 0 12px 32px rgba(0,0,0,0.35);
  }
  .form-row { display: flex; margin: 5px 0; }
  .form-lbl { width: 185px; color: ${t.meta}; }
  .form-lbl.act { color: ${t.title}; font-weight: 700; }
  .form-val { color: ${t.text}; }
  .cursor { display: inline-block; width: 8px; height: 15px; background: ${t.title}; vertical-align: middle; margin-left: 2px; }
</style>
</head>
<body>
  <div class="win">
    <div class="titlebar">
      <div style="display:flex;align-items:center;gap:12px;">
        <div class="tab-pill"><span>&gt;_</span><span>${titleBar}</span></div>
        ${subtitlePill ? `<span class="badge-pill">${subtitlePill}</span>` : ""}
      </div>
      <div class="win-controls"><span>─</span><span>□</span><span>✕</span></div>
    </div>
    <div class="term">
      ${innerHtml}
    </div>
  </div>
</body>
</html>`;
}

function renderDashboardBody(themeName, opts = {}) {
  const t = THEMES[themeName];
  const groupSelected = !!opts.groupSelected;
  const searchQuery = opts.searchQuery || "";
  return `
  <div>
    <div class="row-between">
      <div><span class="title">sshm v1.0.0</span>  <span class="stats">● 6 hosts · 3 groups · 5 online</span></div>
      <div class="meta">[?] Help [a] Add [g] Group [t] Theme [r] Probe [q] Quit</div>
    </div>
    <div class="divider"></div>
    <div class="row-between">
      <div><span class="title">&gt; search:</span> <span>${searchQuery}<span class="cursor"></span></span></div>
      <div>
        <span class="tab-act">All</span>
        <span class="tab-inact">prod</span>
        <span class="tab-inact">staging</span>
        <span class="tab-inact">infra</span>
      </div>
    </div>
  </div>

  <div class="split">
    <div class="left-pane">
      <div class="grp-hdr" style="${groupSelected ? `background:${t.selBg};color:${t.selFg};padding:2px 6px;border-radius:3px;` : ""}">▼ <span style="color:${groupSelected ? t.selFg : t.dotRed}">●</span> prod (3)</div>
      <div class="host-row ${!groupSelected ? "sel" : ""}">  <span style="color:${t.dotRed}">●</span><span class="pin">★</span> prod-api-01   <span class="meta">deploy@10.0.1.12</span>  <span class="pw">[pw]</span>  <span class="online">● 14ms</span> <span class="fwd">⇄1</span></div>
      <div class="host-row">  <span style="color:${t.dotRed}">●</span>  prod-api-02   <span class="meta">deploy@10.0.1.13</span>  <span class="key">[key]</span> <span class="online">● 22ms</span></div>
      <div class="host-row">  <span style="color:${t.dotRed}">●</span>  prod-db-01    <span class="meta">pg@10.0.1.20</span>      <span class="pw">[pw]</span>  <span class="online">● 31ms</span> <span class="fwd">⇄2</span></div>

      <div class="grp-hdr">▼ <span style="color:${t.dotYellow}">●</span> staging (2)</div>
      <div class="host-row">  <span style="color:${t.dotYellow}">●</span>  stg-web-01    <span class="meta">ubuntu@10.0.2.5</span>   <span class="pw">[pw]</span>  <span class="online">● 45ms</span></div>
      <div class="host-row">  <span style="color:${t.dotYellow}">●</span>  stg-worker    <span class="meta">ubuntu@10.0.2.6</span>   <span class="pw">[pw]</span>  <span class="offline">● off</span></div>

      <div class="grp-hdr">▼ <span style="color:${t.dotCyan}">●</span> infra (1)</div>
      <div class="host-row">  <span style="color:${t.dotCyan}">●</span><span class="pin">★</span> bastion-prod  <span class="meta">ops@203.0.113.10</span>  <span class="key">[key]</span> <span class="online">● 9ms</span></div>
    </div>

    <div class="right-pane">
      ${
        groupSelected
          ? `
      <div class="title" style="${t.mono ? "text-decoration:underline" : ""}">Group — prod</div>
      <div class="divider"></div>
      <div class="kv"><div class="kv-k">Hosts:</div><div class="kv-v">3 total (3 online)</div></div>
      <div class="kv"><div class="kv-k">Badge:</div><div class="kv-v"><span style="color:${t.dotRed}">●</span> red</div></div>
      <div class="kv"><div class="kv-k">Def User:</div><div class="kv-v">deploy</div></div>
      <div class="kv"><div class="kv-k">Def Port:</div><div class="kv-v">22</div></div>
      <div class="kv"><div class="kv-k">Def Key:</div><div class="kv-v">~/.ssh/id_ed25519_prod</div></div>
      <div class="kv"><div class="kv-k">Def Jump:</div><div class="kv-v"><span class="jump">bastion-prod</span></div></div>
      <div class="divider" style="margin-top:14px;"></div>
      <div class="title" style="font-size:13.5px;">Quick Keybindings (?)</div>
      <div class="kv"><div class="kv-k">[Enter]</div><div class="kv-v">Connect / Fold group</div></div>
      <div class="kv"><div class="kv-k">[i] / [r]</div><div class="kv-v">Inspector / TCP Probe</div></div>
      <div class="kv"><div class="kv-k">[f] / [t]</div><div class="kv-v">Port Forwards / Theme</div></div>
      <div class="kv"><div class="kv-k">[g] / [p]</div><div class="kv-v">Edit Group / Pin Host</div></div>
      <div class="dim" style="margin-top:10px;">[g] Edit group  [Space] Fold</div>
      `
          : `
      <div class="title" style="${t.mono ? "text-decoration:underline" : ""}">Host — prod-api-01</div>
      <div class="divider"></div>
      <div class="kv"><div class="kv-k">Endpoint:</div><div class="kv-v">deploy@10.0.1.12:22</div></div>
      <div class="kv"><div class="kv-k">Group:</div><div class="kv-v"><span style="color:${t.dotRed}">●</span> prod  <span class="pin">★ Pinned</span></div></div>
      <div class="kv"><div class="kv-k">Auth:</div><div class="kv-v">password (Windows Cred Mgr)</div></div>
      <div class="kv"><div class="kv-k">Jump:</div><div class="kv-v"><span class="jump">via bastion-prod (inherited)</span></div></div>
      <div class="kv"><div class="kv-k">Status:</div><div class="kv-v"><span class="online">● Online (14ms TCP)</span></div></div>
      <div class="kv"><div class="kv-k">Tags:</div><div class="kv-v"><span class="tag">#api</span> <span class="tag">#linux</span> <span class="tag">#primary</span></div></div>
      <div class="kv"><div class="kv-k">Tunnels:</div><div class="kv-v">1 configured ([f] edit)</div></div>
      <div class="fwd" style="padding-left:14px;">L 127.0.0.1:8080 -&gt; 127.0.0.1:80</div>
      <div class="kv" style="margin-top:4px;"><div class="kv-k">Notes:</div><div class="kv-v">Primary production API node</div></div>
      <div class="kv"><div class="kv-k">Last Seen:</div><div class="kv-v">2m ago</div></div>
      <div class="dim" style="margin-top:14px;">[Enter] Connect  [e] Edit  [f] Tunnels</div>
      `
      }
    </div>
  </div>

  <div>
    <div class="divider"></div>
    <div class="meta">[Enter] Connect [Tab] Group [/] Search [i] Inspector [e] Edit [f] Forwards [d] Delete [p] Pin</div>
    <div class="row-between" style="margin-top:4px;">
      <div class="watermark">Copyright © 2026 Oktanetflow</div>
      <div class="meta">Theme: ${themeName} · Store: Windows Credential Manager</div>
    </div>
  </div>`;
}

function renderAddHostModal() {
  return renderWindowShell(
    "Windows Terminal — sshm (Add Host)",
    "minimal-dark",
    `
    <div class="modal-wrap">
      <div class="modal-box">
        <div class="title" style="font-size:17px;margin-bottom:14px;">Add New SSH Host</div>
        <div class="form-row"><div class="form-lbl">Alias:</div><div class="form-val">prod-cache-01</div></div>
        <div class="form-row"><div class="form-lbl">Hostname / IP:</div><div class="form-val">10.0.1.30</div></div>
        <div class="form-row"><div class="form-lbl">Port:</div><div class="form-val">22 <span class="dim">(inherits group default)</span></div></div>
        <div class="form-row"><div class="form-lbl">Username:</div><div class="form-val">deploy</div></div>
        <div class="form-row"><div class="form-lbl">Group:</div><div class="form-val"><span style="color:#FF5F5F">●</span> prod</div></div>
        <div class="form-row"><div class="form-lbl">Auth Type:</div><div class="form-val"><span class="pw">[x] password</span>   <span class="dim">[ ] key   [ ] agent   [ ] auto</span></div></div>
        <div class="form-row"><div class="form-lbl">Save Password:</div><div class="form-val"><span class="online">[x] Yes — Windows Credential Manager (Zero Plaintext)</span></div></div>
        <div class="form-row"><div class="form-lbl act">&gt; Password:</div><div class="form-val">••••••••••••••<span class="cursor"></span></div></div>
        <div class="form-row"><div class="form-lbl">Jump Host:</div><div class="form-val"><span class="jump">bastion-prod</span> <span class="dim">(inherited)</span></div></div>
        <div class="form-row"><div class="form-lbl">Tags:</div><div class="form-val"><span class="tag">redis, cache, prod</span></div></div>
        <div class="form-row"><div class="form-lbl">Notes:</div><div class="form-val">Primary Redis session cluster</div></div>
        <div class="divider" style="margin:14px 0 10px 0;"></div>
        <div class="meta">[Tab/↑↓] Field  [Space/←→] Toggle  [Enter] Save  [Ctrl+S] Save &amp; Connect  [Esc] Cancel</div>
        <div class="watermark" style="margin-top:6px;">Copyright © 2026 Oktanetflow · Password stored securely in Windows Credential Manager</div>
      </div>
    </div>`,
    "Modal · [a] Add Host"
  );
}

function renderForwardsModal() {
  return renderWindowShell(
    "Windows Terminal — sshm (Port Forwards)",
    "minimal-dark",
    `
    <div class="modal-wrap">
      <div class="modal-box">
        <div class="title" style="font-size:17px;margin-bottom:16px;">Port Forwards — prod-db-01</div>
        <div class="host-row sel" style="margin-bottom:4px;">&gt; L 127.0.0.1:5432 -&gt; 127.0.0.1:5432</div>
        <div class="host-row" style="margin-bottom:4px;">  D 127.0.0.1:1080 (SOCKS5)</div>
        <div class="host-row" style="margin-bottom:14px;">  R 127.0.0.1:9000 -&gt; 127.0.0.1:3000</div>

        <div class="divider"></div>
        <div style="margin:12px 0;">
          <div class="form-row"><div class="form-lbl">Type :</div><div class="form-val"><span class="title">[x] local</span>   <span class="dim">[ ] remote</span>   <span class="dim">[ ] dynamic</span></div></div>
          <div class="form-row"><div class="form-lbl act">Spec :</div><div class="form-val">8080:10.0.1.12:80<span class="cursor"></span>  <span class="dim">(e.g. 5432:127.0.0.1:5432)</span></div></div>
        </div>

        <div style="background:#142C1F;border:1px solid #87D787;border-radius:6px;padding:8px 12px;margin:10px 0;font-size:13px;color:#87D787;">
          ✓ Persistent tunnels auto-start on every 'sshm prod-db-01' session (IPv4 &amp; [IPv6] supported)
        </div>

        <div class="meta" style="margin-top:12px;">[Tab/←→] Type  [Enter] Save Tunnel  [d] Remove  [Esc] Back</div>
        <div class="watermark" style="margin-top:6px;">Copyright © 2026 Oktanetflow</div>
      </div>
    </div>`,
    "Modal · [f] Port Forwards"
  );
}

const MOCKUPS = [
  {
    name: "tui-minimal-dark.png",
    html: renderWindowShell("Windows Terminal — sshm", "minimal-dark", renderDashboardBody("minimal-dark"), "Theme: minimal-dark"),
  },
  {
    name: "tui-minimal-light.png",
    html: renderWindowShell("Windows Terminal — sshm", "minimal-light", renderDashboardBody("minimal-light"), "Theme: minimal-light"),
  },
  {
    name: "tui-mono.png",
    html: renderWindowShell("Windows Terminal — sshm", "mono", renderDashboardBody("mono"), "Theme: mono"),
  },
  {
    name: "tui-add-host-modal.png",
    html: renderAddHostModal(),
  },
  {
    name: "tui-port-forwards-modal.png",
    html: renderForwardsModal(),
  },
  {
    name: "tui-help-group-modal.png",
    html: renderWindowShell("Windows Terminal — sshm", "minimal-dark", renderDashboardBody("minimal-dark", { groupSelected: true }), "Group Inspector & Help"),
  },
];

// ---------------------------------------------------------------------------
// Illustrated How-To Video Studio HTML (Renders Canvas -> WebM + GIF Frames)
// ---------------------------------------------------------------------------
function buildVideoStudioHtml() {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8"/>
<title>sshm Illustrated How-To Video Renderer</title>
</head>
<body style="margin:0;background:#0D1117;">
<canvas id="c" width="960" height="540"></canvas>
<script>
const W = 960, H = 540;
const canvas = document.getElementById('c');
const ctx = canvas.getContext('2d', { willReadFrequently: true });

function loadImage(url) {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => resolve(img);
    img.onerror = reject;
    img.src = url;
  });
}

function roundRect(x, y, w, h, r, fill, stroke, lw = 1.5) {
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, r);
  if (fill) { ctx.fillStyle = fill; ctx.fill(); }
  if (stroke) { ctx.strokeStyle = stroke; ctx.lineWidth = lw; ctx.stroke(); }
}

function drawKeycap(x, y, label, active = false) {
  ctx.font = 'bold 13px "Cascadia Mono", Consolas, monospace';
  const tw = ctx.measureText(label).width;
  const pw = tw + 18;
  roundRect(x, y, pw, 26, 6, active ? '#5FD7FF' : '#1E2630', active ? '#5FD7FF' : '#38465A', 1.5);
  ctx.fillStyle = active ? '#0D1117' : '#D0D0D0';
  ctx.fillText(label, x + 9, y + 17);
  return pw;
}

function drawStepHeader(demoTag, stepNum, totalSteps, title, keyLabel) {
  // Top bar
  roundRect(0, 0, W, 64, 0, '#131A24', null);
  ctx.strokeStyle = '#263040';
  ctx.lineWidth = 1.5;
  ctx.beginPath(); ctx.moveTo(0, 64); ctx.lineTo(W, 64); ctx.stroke();

  // Dynamic width badge
  ctx.font = 'bold 12px "Segoe UI", sans-serif';
  const badgeW = Math.ceil(ctx.measureText(demoTag).width) + 28;
  roundRect(20, 18, badgeW, 28, 14, '#112233', '#5FD7FF', 1.2);
  ctx.fillStyle = '#5FD7FF';
  ctx.fillText(demoTag, 34, 36);

  // Step title positioned cleanly after badge
  ctx.font = 'bold 17.5px "Segoe UI", sans-serif';
  ctx.fillStyle = '#F0F6FC';
  ctx.fillText('Step ' + stepNum + '/' + totalSteps + ': ' + title, 20 + badgeW + 14, 38);

  // Keycap hint on right
  if (keyLabel) {
    ctx.font = 'bold 13px "Cascadia Mono", Consolas, monospace';
    const kw = ctx.measureText(keyLabel).width + 20;
    drawKeycap(W - kw - 22, 19, keyLabel, true);
  }

  // Progress bar
  const pct = stepNum / totalSteps;
  roundRect(0, 61, W, 3, 0, '#1E2630', null);
  roundRect(0, 61, Math.round(W * pct), 3, 0, '#5FD7FF', null);
}

function drawFooter(note) {
  roundRect(0, H - 32, W, 32, 0, '#10161F', null);
  ctx.font = '12px "Segoe UI", sans-serif';
  ctx.fillStyle = '#8B949E';
  ctx.fillText(note, 20, H - 11);
  ctx.fillStyle = '#5FD7FF';
  ctx.fillText('github.com/oktaaa17/sshm  ·  Copyright © 2026 Oktanetflow', W - 365, H - 11);
}

function drawIntroSlide(img, badge, headline, steps) {
  ctx.fillStyle = '#0D1117';
  ctx.fillRect(0, 0, W, H);
  // Draw illustration cleanly inside upper showcase frame
  const iw = W - 48, ih = 356;
  ctx.save();
  ctx.beginPath();
  ctx.roundRect(24, 20, iw, ih, 14);
  ctx.clip();
  ctx.drawImage(img, 24, 20, iw, ih);
  // Subtle bottom gradient vignette on illustration
  const grad = ctx.createLinearGradient(0, 260, 0, 376);
  grad.addColorStop(0, 'rgba(13,17,23,0)');
  grad.addColorStop(1, 'rgba(13,17,23,0.92)');
  ctx.fillStyle = grad;
  ctx.fillRect(24, 260, iw, 120);
  ctx.restore();
  roundRect(24, 20, iw, ih, 14, null, '#5FD7FF', 1.6);

  // Overlay badge on top-left
  roundRect(42, 36, 260, 30, 15, 'rgba(13,17,23,0.88)', '#5FD7FF', 1.4);
  ctx.font = 'bold 12.5px "Segoe UI", sans-serif';
  ctx.fillStyle = '#5FD7FF';
  ctx.fillText(badge, 56, 55);

  // Headline at bottom of illustration
  ctx.font = 'bold 23px "Segoe UI", sans-serif';
  ctx.fillStyle = '#F0F6FC';
  ctx.fillText(headline, 44, 355);

  // 4 Illustrated Step Cards along bottom
  const cardW = (W - 48 - 36) / steps.length;
  steps.forEach((s, idx) => {
    const cx = 24 + idx * (cardW + 12);
    roundRect(cx, 394, cardW, 96, 10, '#161B22', s.color, 1.5);
    roundRect(cx + 12, 406, 56, 20, 10, '#112233', null);
    ctx.font = 'bold 11px "Segoe UI", sans-serif';
    ctx.fillStyle = s.color;
    ctx.fillText('STEP ' + (idx + 1), cx + 21, 420);
    ctx.font = 'bold 14px "Segoe UI", sans-serif';
    ctx.fillStyle = '#F0F6FC';
    ctx.fillText(s.title, cx + 12, 448);
    ctx.font = '12px "Segoe UI", sans-serif';
    ctx.fillStyle = '#8B949E';
    ctx.fillText(s.sub, cx + 12, 470);
  });

  drawFooter('Illustrated How-To Walkthrough · Powered by sshm v1.0.0');
}

function drawTerminalFrame(x, y, w, h, title, theme = 'dark') {
  const isLight = theme === 'light';
  const isMono = theme === 'mono';
  const bg = isLight ? '#FAFBFD' : (isMono ? '#111111' : '#0E1117');
  const bar = isLight ? '#EAEFF5' : (isMono ? '#1C1C1C' : '#161B22');
  const bdr = isLight ? '#CBD5E1' : (isMono ? '#4A4A4A' : '#2D394B');
  roundRect(x, y, w, h, 10, bg, bdr, 1.5);
  ctx.save();
  ctx.beginPath(); ctx.roundRect(x, y, w, 30, [10, 10, 0, 0]); ctx.clip();
  ctx.fillStyle = bar; ctx.fillRect(x, y, w, 30);
  ctx.restore();
  ctx.strokeStyle = bdr; ctx.lineWidth = 1;
  ctx.beginPath(); ctx.moveTo(x, y + 30); ctx.lineTo(x + w, y + 30); ctx.stroke();
  ctx.font = 'bold 12px "Segoe UI", sans-serif';
  ctx.fillStyle = isLight ? '#005F87' : (isMono ? '#E6E6E6' : '#5FD7FF');
  ctx.fillText('>_  ' + title, x + 14, y + 19);
  ctx.fillStyle = '#808080';
  ctx.fillText('─   □   ✕', x + w - 68, y + 19);
}

// ---------------- DEMO 1: Agile TUI Workflow ----------------
function buildDemo1Frames(imgs) {
  const frames = [];
  const push = (drawFn, delayMs) => frames.push({ drawFn, delayMs });

  // Scene 1: Intro using hero-banner.png
  push(() => drawIntroSlide(
    imgs.hero,
    'HOW TO USE sshm · DEMO 1 OF 3',
    'Interactive Agile TUI: Live Probes, Split Inspector, Search & Themes',
    [
      { title: 'Launch sshm', sub: 'Instant auto-probe', color: '#5FD7FF' },
      { title: 'Inspect [i]', sub: 'Inherited defaults', color: '#87D787' },
      { title: 'Search [/]', sub: 'Filter alias & tags', color: '#AF87FF' },
      { title: 'Theme [t]', sub: 'Dark · Light · Mono', color: '#FFAF5F' },
    ]
  ), 2200);

  const drawTuiScene = ({ step, title, keyHint, probesReady, selectedIdx, search, theme, calloutTitle, calloutLines, calloutColor }) => {
    ctx.fillStyle = '#0D1117';
    ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · TUI', step, 5, title, keyHint);

    // Left illustrated explainer rail (220px wide)
    roundRect(20, 82, 228, 408, 12, '#161B22', calloutColor, 1.5);
    roundRect(34, 96, 200, 108, 8, '#0D1117', '#263040', 1);
    ctx.save();
    ctx.beginPath(); ctx.roundRect(34, 96, 200, 108, 8); ctx.clip();
    ctx.drawImage(imgs.hero, 34, 96, 200, 108);
    ctx.restore();

    ctx.font = 'bold 14px "Segoe UI", sans-serif';
    ctx.fillStyle = calloutColor;
    ctx.fillText(calloutTitle, 34, 228);

    ctx.font = '12.5px "Segoe UI", sans-serif';
    calloutLines.forEach((ln, i) => {
      ctx.fillStyle = i === 0 ? '#F0F6FC' : '#9AA4B2';
      ctx.fillText(ln, 34, 254 + i * 22);
    });

    // Visual status pill at bottom of rail
    roundRect(34, 434, 200, 40, 8, '#112233', calloutColor, 1.2);
    ctx.font = 'bold 12px "Cascadia Mono", Consolas, monospace';
    ctx.fillStyle = calloutColor;
    ctx.fillText('Key: ' + keyHint, 48, 458);

    // Right live TUI window (680px wide)
    const tx = 264, ty = 82, tw = 676, th = 408;
    const isLight = theme === 'light';
    const isMono = theme === 'mono';
    drawTerminalFrame(tx, ty, tw, th, 'Windows Terminal — sshm (' + (isLight ? 'minimal-light' : isMono ? 'mono' : 'minimal-dark') + ')', theme);

    const fg = isLight ? '#262626' : (isMono ? '#E6E6E6' : '#D0D0D0');
    const muted = isLight ? '#767676' : '#808080';
    const acc = isLight ? '#005F87' : (isMono ? '#F0F0F0' : '#5FD7FF');
    const green = isLight ? '#008700' : (isMono ? '#FFFFFF' : '#87D787');
    const red = isLight ? '#D70000' : (isMono ? '#888888' : '#FF5F5F');
    const amber = isLight ? '#AF5F00' : (isMono ? '#B0B0B0' : '#FFAF5F');
    const selBg = acc;
    const selFg = isLight ? '#FFFFFF' : '#0E1117';

    ctx.font = 'bold 13px "Cascadia Mono", Consolas, monospace';
    ctx.fillStyle = acc;
    ctx.fillText('sshm v1.0.0', tx + 14, ty + 52);
    ctx.font = '12.5px "Cascadia Mono", Consolas, monospace';
    ctx.fillStyle = muted;
    ctx.fillText(probesReady ? '5 hosts · 2 groups · 4 online' : '5 hosts · 2 groups · probing...', tx + 114, ty + 52);
    ctx.fillText('[?] [a] [f] [t] [r] [q]', tx + tw - 188, ty + 52);

    ctx.strokeStyle = isLight ? '#CBD5E1' : '#333C4A';
    ctx.beginPath(); ctx.moveTo(tx + 14, ty + 62); ctx.lineTo(tx + tw - 14, ty + 62); ctx.stroke();

    ctx.fillStyle = acc;
    ctx.fillText('> search: ', tx + 14, ty + 82);
    ctx.fillStyle = fg;
    ctx.fillText(search + '▌', tx + 92, ty + 82);
    roundRect(tx + tw - 196, ty + 69, 36, 18, 3, acc, null);
    ctx.fillStyle = selFg; ctx.font = 'bold 11.5px "Cascadia Mono", monospace';
    ctx.fillText('All', tx + tw - 189, ty + 82);
    ctx.fillStyle = muted; ctx.font = '11.5px "Cascadia Mono", monospace';
    ctx.fillText('prod   staging', tx + tw - 150, ty + 82);

    const hosts = [
      { alias: 'prod-api-01', ep: 'deploy@10.0.1.12', auth: '[pw]', ms: '● 14ms', ok: true, group: 'prod', pin: true, tags: '#api #linux', fwd: 'L :8080 -> :80', notes: 'Primary prod API' },
      { alias: 'prod-api-02', ep: 'deploy@10.0.1.13', auth: '[key]', ms: '● 22ms', ok: true, group: 'prod', pin: false, tags: '#api #linux', fwd: 'none', notes: 'Secondary API' },
      { alias: 'prod-db-01',  ep: 'pg@10.0.1.20',     auth: '[pw]', ms: '● 31ms', ok: true, group: 'prod', pin: false, tags: '#postgres #db', fwd: 'L :5432 -> :5432', notes: 'Primary Postgres' },
      { alias: 'stg-web-01',  ep: 'ubuntu@10.0.2.5',  auth: '[pw]', ms: '● 45ms', ok: true, group: 'staging', pin: false, tags: '#web', fwd: 'none', notes: 'Staging frontend' },
      { alias: 'stg-worker',  ep: 'ubuntu@10.0.2.6',  auth: '[pw]', ms: '● off',  ok: false, group: 'staging', pin: false, tags: '#jobs', fwd: 'none', notes: 'Scaled down' },
    ];
    const filtered = search ? hosts.filter(h => (h.alias + h.tags + h.notes).includes(search)) : hosts;
    const selHost = filtered[Math.min(selectedIdx, filtered.length - 1)] || hosts[0];

    // Left list pane
    let ly = ty + 110;
    ctx.font = 'bold 12.5px "Cascadia Mono", monospace';
    ctx.fillStyle = fg;
    ctx.fillText('▼ prod / staging (' + filtered.length + ')', tx + 14, ly);
    ly += 22;

    filtered.forEach((h, i) => {
      const isSel = i === Math.min(selectedIdx, filtered.length - 1);
      if (isSel) {
        roundRect(tx + 12, ly - 14, 358, 22, 3, selBg, null);
      }
      ctx.font = (isSel ? 'bold ' : '') + '12px "Cascadia Mono", monospace';
      ctx.fillStyle = isSel ? selFg : (h.group === 'prod' ? red : amber);
      ctx.fillText('●' + (h.pin ? '★ ' : '  ') + h.alias.padEnd(13), tx + 18, ly);
      ctx.fillStyle = isSel ? selFg : muted;
      ctx.fillText(h.ep.slice(0, 17).padEnd(18), tx + 132, ly);
      ctx.fillStyle = isSel ? selFg : (!probesReady ? amber : (h.ok ? green : red));
      ctx.fillText(probesReady ? h.ms : '○ ...', tx + 298, ly);
      ly += 26;
    });

    // Right Split-Pane Inspector
    const ix = tx + 382;
    ctx.strokeStyle = isLight ? '#CBD5E1' : '#333C4A';
    ctx.beginPath(); ctx.moveTo(ix, ty + 96); ctx.lineTo(ix, ty + th - 58); ctx.stroke();

    ctx.font = 'bold 13px "Cascadia Mono", monospace';
    ctx.fillStyle = acc;
    ctx.fillText('Host — ' + selHost.alias, ix + 14, ty + 114);
    ctx.strokeStyle = isLight ? '#CBD5E1' : '#333C4A';
    ctx.beginPath(); ctx.moveTo(ix + 14, ty + 124); ctx.lineTo(tx + tw - 14, ty + 124); ctx.stroke();

    const rows = [
      ['Endpoint:', selHost.ep + ':22'],
      ['Group:', selHost.group + (selHost.pin ? ' ★ Pinned' : '')],
      ['Auth:', selHost.auth === '[pw]' ? 'password (WinCred)' : 'key (id_ed25519)'],
      ['Jump:', 'via bastion-prod (inh.)'],
      ['Status:', probesReady ? (selHost.ok ? 'Online (' + selHost.ms.slice(2) + ')' : 'Offline') : 'Checking TCP...'],
      ['Tags:', selHost.tags],
      ['Tunnels:', selHost.fwd],
      ['Notes:', selHost.notes],
    ];
    rows.forEach(([k, v], idx) => {
      const ry = ty + 146 + idx * 24;
      ctx.font = '11.8px "Cascadia Mono", monospace';
      ctx.fillStyle = muted;
      ctx.fillText(k, ix + 14, ry);
      ctx.fillStyle = k === 'Status:' && probesReady ? (selHost.ok ? green : red) : fg;
      ctx.fillText(v, ix + 92, ry);
    });

    // Footer inside terminal
    ctx.strokeStyle = isLight ? '#CBD5E1' : '#333C4A';
    ctx.beginPath(); ctx.moveTo(tx + 14, ty + th - 48); ctx.lineTo(tx + tw - 14, ty + th - 48); ctx.stroke();
    ctx.font = '11.5px "Cascadia Mono", monospace';
    ctx.fillStyle = muted;
    ctx.fillText('[Enter] Connect  [/] Search  [i] Inspector  [f] Forwards  [t] Theme  [r] Probe', tx + 14, ty + th - 30);
    ctx.fillStyle = acc;
    ctx.fillText('Copyright © 2026 Oktanetflow · Theme: ' + (isLight ? 'minimal-light' : isMono ? 'mono' : 'minimal-dark'), tx + 14, ty + th - 12);

    drawFooter('Demo 1/3 · Run "sshm" in Windows Terminal or PowerShell to start the interactive TUI');
  };

  // Step 1: Launching & Async Probing
  push(() => drawTuiScene({
    step: 1, title: 'Launch sshm — Non-Blocking TCP Reachability Probe', keyHint: 'sshm / [r]',
    probesReady: false, selectedIdx: 0, search: '', theme: 'dark',
    calloutTitle: 'Async TCP Probing', calloutColor: '#FFD75F',
    calloutLines: ['Startup never stalls!', 'Bounded goroutine pool', 'checks host:port in', 'parallel (1500ms timeout)', 'while UI stays instant.', '', 'Press [r] anytime to', 'refresh live latency.'],
  }), 1200);
  push(() => drawTuiScene({
    step: 1, title: 'Live Latency Badges Appear Automatically', keyHint: '[r] Probe',
    probesReady: true, selectedIdx: 0, search: '', theme: 'dark',
    calloutTitle: 'Live Status Ready', calloutColor: '#87D787',
    calloutLines: ['4 of 5 hosts online!', 'Green ● 14ms = reachable', 'Red ● off = unreachable', 'Header counter updates', 'automatically.', '', 'Disable auto-probe via:', 'auto_probe_on_start false'],
  }), 1500);

  // Step 2: Navigate & Split-Pane Inspector
  push(() => drawTuiScene({
    step: 2, title: 'Inspect Effective Defaults in Split-Pane Inspector', keyHint: '[↓] / [i]',
    probesReady: true, selectedIdx: 2, search: '', theme: 'dark',
    calloutTitle: 'Split-Pane Inspector', calloutColor: '#5FD7FF',
    calloutLines: ['Auto-opens on terminals', '≥ 100 columns wide.', '', 'Shows inherited group', 'jump host, auth backend,', 'tags, port forwards,', 'notes & last-seen time.', '', 'Toggle anytime with [i].'],
  }), 1800);

  // Step 3: Fuzzy Search typing animation
  for (const q of ['p', 'pr', 'prod', 'prod-d', 'prod-db']) {
    push(() => drawTuiScene({
      step: 3, title: 'Fuzzy Search Across Alias, Host, Tags & Notes', keyHint: '[/] Search',
      probesReady: true, selectedIdx: 0, search: q, theme: 'dark',
      calloutTitle: 'Instant Fuzzy Filter', calloutColor: '#AF87FF',
      calloutLines: ['Press [/] to filter live.', '', 'Matches:', '• Alias (prod-db-01)', '• IP / Hostname', '• Group & Jump Host', '• #Tags & Notes', '', 'Hit Enter to connect!'],
    }), q === 'prod-db' ? 1400 : 280);
  }

  // Step 4: Live Theme Switcher
  push(() => drawTuiScene({
    step: 4, title: 'Cycle Built-In Themes Live (minimal-light)', keyHint: '[t] Theme',
    probesReady: true, selectedIdx: 0, search: '', theme: 'light',
    calloutTitle: 'minimal-light Theme', calloutColor: '#5FD7FF',
    calloutLines: ['Press [t] to cycle live:', '1. minimal-dark', '2. minimal-light  ◄ NOW', '3. mono (accessible)', '', 'Selection persists', 'automatically into', 'config.toml!'],
  }), 1400);
  push(() => drawTuiScene({
    step: 4, title: 'Cycle Built-In Themes Live (mono Accessible Mode)', keyHint: '[t] Theme',
    probesReady: true, selectedIdx: 0, search: '', theme: 'mono',
    calloutTitle: 'mono High-Contrast', calloutColor: '#E6E6E6',
    calloutLines: ['Zero-color distraction,', 'crisp reverse-video', 'selection for high-', 'contrast & monochrome', 'Windows terminals.', '', 'Press [t] once more', 'to return to dark.'],
  }), 1300);

  // Step 5: Connected native Win32 VT100 SSH shell
  push(() => {
    ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · TUI', 5, 5, 'Press [Enter] — Native Win32 VT100 SSH Session', '[Enter]');
    roundRect(20, 82, 228, 408, 12, '#161B22', '#87D787', 1.5);
    ctx.font = 'bold 15px "Segoe UI", sans-serif'; ctx.fillStyle = '#87D787';
    ctx.fillText('Pure-Go SSHv2 Engine', 34, 118);
    const pts = ['• Zero ssh.exe / PuTTY', '• Auto ProxyJump chain', '  via bastion-prod', '• Auto WinCred auth', '• Native Win32 VT100', '  raw mode + resize', '• Known-hosts TOFU', '  MITM verification', '', 'Type "exit" to return', 'back to sshm cleanly.'];
    ctx.font = '12.5px "Segoe UI", sans-serif';
    pts.forEach((p, i) => { ctx.fillStyle = i < 1 ? '#F0F6FC' : '#C9D1D9'; ctx.fillText(p, 34, 150 + i * 24); });

    drawTerminalFrame(264, 82, 676, 408, 'sshm — connected: deploy@prod-api-01 (via bastion-prod)', 'dark');
    const lines = [
      ['#5FD7FF', 'Connecting to prod-api-01 (10.0.1.12:22) via bastion-prod...'],
      ['#87D787', '✓ Host key verified against ~/.ssh/known_hosts (ed25519)'],
      ['#87D787', '✓ Authenticated via Windows Credential Manager (0 plaintext)'],
      ['#AF87FF', '⇄ Active Tunnel: L 127.0.0.1:8080 -> 127.0.0.1:80'],
      ['#808080', '──────────────────────────────────────────────────────────────'],
      ['#D0D0D0', 'Welcome to Ubuntu 24.04 LTS (GNU/Linux 6.8.0 x86_64)'],
      ['#D0D0D0', ''],
      ['#87D787', 'deploy@prod-api-01:~$ '],
      ['#F0F6FC', '  systemctl status api-gateway --no-pager | head -n 3'],
      ['#87D787', '● api-gateway.service - Oktanetflow Production API'],
      ['#D0D0D0', '     Loaded: loaded (/etc/systemd/system/api-gateway.service)'],
      ['#87D787', '     Active: active (running) since Thu 2026-11-26 09:00 UTC'],
      ['#87D787', 'deploy@prod-api-01:~$ ▌'],
    ];
    ctx.font = '13px "Cascadia Mono", Consolas, monospace';
    lines.forEach(([col, txt], i) => {
      ctx.fillStyle = col;
      ctx.fillText(txt, 282, 138 + i * 26);
    });
    drawFooter('Demo 1/3 Complete · See docs/MOCKUPS.md & docs/USAGE.md for every shortcut');
  }, 2000);

  return frames;
}

// ---------------- DEMO 2: Add Host & Windows Credential Manager ----------------
function buildDemo2Frames(imgs) {
  const frames = [];
  const push = (drawFn, delayMs) => frames.push({ drawFn, delayMs });

  // Scene 1: Intro using security-credential-vault.png
  push(() => drawIntroSlide(
    imgs.vault,
    'HOW TO USE sshm · DEMO 2 OF 3',
    'Add Host & Save Password Safely into Windows Credential Manager',
    [
      { title: 'Press [a]', sub: 'Open Add Host form', color: '#5FD7FF' },
      { title: 'Save PW: ON', sub: 'Default secure store', color: '#FFAF5F' },
      { title: 'Zero Plaintext', sub: 'ID ref only in TOML', color: '#87D787' },
      { title: '1-Key Connect', sub: 'Auto WinCred login', color: '#AF87FF' },
    ]
  ), 2200);

  const drawFormStep = (step, title, keyHint, vals, activeField, showSplitFlow) => {
    ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · CREDENTIALS', step, 4, title, keyHint);

    if (!showSplitFlow) {
      // Left: Add Host modal
      drawTerminalFrame(22, 82, 590, 408, 'Windows Terminal — sshm [a] Add Host', 'dark');
      roundRect(42, 126, 550, 344, 10, '#121720', '#5FD7FF', 1.5);
      ctx.font = 'bold 16px "Cascadia Mono", monospace'; ctx.fillStyle = '#5FD7FF';
      ctx.fillText('Add New SSH Host', 60, 156);

      const fields = [
        ['Alias', vals.alias],
        ['Hostname', vals.host],
        ['Username', vals.user],
        ['Group', vals.group],
        ['Auth Type', '[x] password   [ ] key   [ ] agent'],
        ['Save Password', '[x] Yes — Windows Credential Manager'],
        ['Password', vals.pw],
        ['Tags', vals.tags],
        ['Notes', vals.notes],
      ];
      fields.forEach(([lbl, val], idx) => {
        const y = 190 + idx * 28;
        const isAct = lbl === activeField;
        ctx.font = (isAct ? 'bold ' : '') + '12.5px "Cascadia Mono", monospace';
        ctx.fillStyle = isAct ? '#5FD7FF' : '#808080';
        ctx.fillText((isAct ? '> ' : '  ') + lbl.padEnd(14) + ':', 58, y);
        ctx.fillStyle = lbl === 'Save Password' ? '#87D787' : (lbl === 'Auth Type' ? '#FFAF5F' : '#F0F6FC');
        ctx.fillText(val + (isAct ? '▌' : ''), 214, y);
      });
      ctx.font = '11.5px "Cascadia Mono", monospace'; ctx.fillStyle = '#808080';
      ctx.fillText('[Tab] Next   [Enter] Save   [Ctrl+S] Save & Connect', 60, 452);

      // Right: Illustrated Security Architecture preview card
      roundRect(630, 82, 310, 408, 12, '#161B22', '#87D787', 1.5);
      ctx.save();
      ctx.beginPath(); ctx.roundRect(644, 96, 282, 158, 8); ctx.clip();
      ctx.drawImage(imgs.vault, 644, 96, 282, 158);
      ctx.restore();
      roundRect(644, 96, 282, 158, 8, null, '#263040', 1);

      ctx.font = 'bold 14.5px "Segoe UI", sans-serif'; ctx.fillStyle = '#87D787';
      ctx.fillText('Save Password by Default', 646, 282);
      const notes = [
        '• Password input is masked',
        '  and never logged to history.',
        '• Written directly to Windows',
        '  Credential Manager (WinCred).',
        '• hosts.toml stores ONLY an',
        '  opaque ID reference.',
        '• Toggle off with Space or',
        '  CLI flag --no-save-password.',
      ];
      ctx.font = '12.5px "Segoe UI", sans-serif';
      notes.forEach((n, i) => { ctx.fillStyle = '#C9D1D9'; ctx.fillText(n, 646, 308 + i * 21); });
    } else {
      // Illustrated Under-the-Hood Split Save Visual
      ctx.font = 'bold 17px "Segoe UI", sans-serif'; ctx.fillStyle = '#F0F6FC';
      ctx.fillText('What Happens the Instant You Press [Enter]:', 28, 102);

      // Left box: hosts.toml (Non-sensitive metadata)
      roundRect(28, 122, 420, 210, 12, '#161B22', '#87D787', 1.6);
      ctx.font = 'bold 14px "Cascadia Mono", monospace'; ctx.fillStyle = '#87D787';
      ctx.fillText('1. %APPDATA%\\sshm\\hosts.toml (Owner-Only NTFS ACL)', 44, 150);
      const tomlLines = [
        '[[hosts]]',
        'id             = "h_9f4a21c8"',
        'alias          = "prod-cache-01"',
        'hostname       = "10.0.1.30"',
        'group          = "prod"',
        'auth           = "password"',
        'credential_ref = "sshm:h_9f4a21c8"  # NO PLAINTEXT!',
      ];
      ctx.font = '12px "Cascadia Mono", monospace';
      tomlLines.forEach((l, i) => {
        ctx.fillStyle = i === 6 ? '#87D787' : '#C9D1D9';
        ctx.fillText(l, 44, 180 + i * 21);
      });

      // Right box: Windows Credential Manager
      roundRect(512, 122, 420, 210, 12, '#161B22', '#5FD7FF', 1.6);
      ctx.font = 'bold 14px "Cascadia Mono", monospace'; ctx.fillStyle = '#5FD7FF';
      ctx.fillText('2. Windows Credential Manager (CredWriteW)', 528, 150);
      const credLines = [
        'TargetName  : sshm:h_9f4a21c8',
        'Type        : CRED_TYPE_GENERIC',
        'Persist     : CRED_PERSIST_LOCAL_MACHINE',
        'SecretBlob  : [OS-Encrypted Credential]',
        '',
        '✓ RAM password slice zeroed after write',
        '✓ Also supports DPAPI & Argon2id Vault',
      ];
      ctx.font = '12px "Cascadia Mono", monospace';
      credLines.forEach((l, i) => {
        ctx.fillStyle = i >= 5 ? '#87D787' : '#C9D1D9';
        ctx.fillText(l, 528, 180 + i * 21);
      });

      // Center connector arrow
      ctx.strokeStyle = '#5FD7FF'; ctx.lineWidth = 3;
      ctx.beginPath(); ctx.moveTo(454, 226); ctx.lineTo(506, 226); ctx.stroke();
      ctx.fillStyle = '#5FD7FF';
      ctx.beginPath(); ctx.moveTo(508, 226); ctx.lineTo(496, 219); ctx.lineTo(496, 233); ctx.fill();

      // Bottom PowerShell verification strip
      drawTerminalFrame(28, 350, 904, 140, 'PowerShell — Verify Zero Plaintext & Instant Reconnect', 'dark');
      ctx.font = '12.5px "Cascadia Mono", monospace';
      ctx.fillStyle = '#5FD7FF'; ctx.fillText('PS C:\\> sshm prod-cache-01', 46, 405);
      ctx.fillStyle = '#87D787'; ctx.fillText('✓ Loaded credential sshm:h_9f4a21c8 from Windows Credential Manager (no re-typing!)', 46, 430);
      ctx.fillStyle = '#F0F6FC'; ctx.fillText('deploy@prod-cache-01:~$ redis-cli ping  =>  PONG', 46, 456);
    }

    drawFooter('Demo 2/3 · See ADR-002 & SECURITY.md for full credential security architecture');
  };

  // Step 1: Open form
  push(() => drawFormStep(1, 'Press [a] to Add a Host (Save Password Enabled by Default)', '[a] Add',
    { alias: 'prod-cache-01', host: '10.0.1.30', user: 'deploy', group: 'prod', pw: '', tags: '', notes: '' }, 'Alias', false), 1400);

  // Step 2: Typing masked password + tags + notes
  for (const pw of ['•••', '•••••••', '••••••••••••']) {
    push(() => drawFormStep(2, 'Enter Masked Password, Tags & Notes', 'Typing...',
      { alias: 'prod-cache-01', host: '10.0.1.30', user: 'deploy', group: 'prod', pw, tags: 'redis,cache', notes: 'Primary Redis cluster' }, 'Password', false), pw.length === 12 ? 1200 : 350);
  }

  // Step 3 & 4: Illustrated Split Save + Instant Connect
  push(() => drawFormStep(3, 'Zero-Plaintext Split Save: TOML Ref + Windows Credential Manager', '[Enter] Save',
    {}, '', true), 2600);

  // Final CLI alternative slide
  push(() => {
    ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · CREDENTIALS', 4, 4, 'Equivalent One-Line CLI Workflow & Backend Switching', 'CLI Mode');
    drawTerminalFrame(24, 86, 912, 398, 'PowerShell 7 — Fast CLI Host Management & Credential Store', 'dark');
    const cmds = [
      ['#808080', '# 1. Add host via CLI (prompts masked password -> saves to Windows Credential Manager)'],
      ['#5FD7FF', 'PS> sshm add prod-cache-01 --host 10.0.1.30 --user deploy --group prod --tags redis,cache'],
      ['#87D787', 'Password: ************'],
      ['#87D787', '✓ Saved host "prod-cache-01" (credential stored in Windows Credential Manager)'],
      ['#D0D0D0', ''],
      ['#808080', '# 2. Switch credential backend anytime (wincred | dpapi | vault)'],
      ['#5FD7FF', 'PS> sshm config set credential_backend vault'],
      ['#87D787', '✓ Updated credential_backend = vault (Argon2id + AES-256-GCM)'],
      ['#D0D0D0', ''],
      ['#808080', '# 3. Connect immediately by alias — zero external ssh.exe needed'],
      ['#5FD7FF', 'PS> sshm prod-cache-01'],
      ['#87D787', '✓ Connected to deploy@10.0.1.30:22 via bastion-prod'],
    ];
    ctx.font = '13px "Cascadia Mono", Consolas, monospace';
    cmds.forEach(([c, l], i) => { ctx.fillStyle = c; ctx.fillText(l, 42, 144 + i * 27); });
    drawFooter('Demo 2/3 Complete · Switch backends anytime via "sshm config set credential_backend"');
  }, 2200);

  return frames;
}

// ---------------- DEMO 3: Port Forwards, Parallel Batch Exec & SFTP ----------------
function buildDemo3Frames(imgs) {
  const frames = [];
  const push = (drawFn, delayMs) => frames.push({ drawFn, delayMs });

  // Scene 1: Intro using network-proxyjump-tunnels.png
  push(() => drawIntroSlide(
    imgs.network,
    'HOW TO USE sshm · DEMO 3 OF 3',
    'Port Forwards (-L/-R/-D), Parallel @group Exec & Built-in SFTP cp',
    [
      { title: 'Tunnels [f]', sub: '-L, -R & SOCKS5 -D', color: '#5FD7FF' },
      { title: 'Parallel Exec', sub: 'sshm exec @prod', color: '#87D787' },
      { title: 'SFTP Copy', sub: 'sshm cp native', color: '#AF87FF' },
      { title: 'Enc Backup', sub: '.sshm.enc vault', color: '#FFAF5F' },
    ]
  ), 2200);

  // Scene 2: Step 1/4 Port Forward Modal + Mini Topology Illustration
  for (const added of [false, true]) {
    push(() => {
      ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
      drawStepHeader('HOW-TO · POWER OPS', 1, 4, 'Configure Persistent Tunnels Inside TUI ([f] Modal)', added ? '[Enter] Saved!' : '[f] Forwards');

      drawTerminalFrame(22, 82, 570, 408, 'sshm — Port Forwards Manager (prod-db-01)', 'dark');
      roundRect(42, 128, 530, 342, 10, '#121720', '#AF87FF', 1.5);
      ctx.font = 'bold 16px "Cascadia Mono", monospace'; ctx.fillStyle = '#5FD7FF';
      ctx.fillText('Port Forwards — prod-db-01', 60, 158);

      roundRect(56, 174, 500, 26, 4, '#5FD7FF', null);
      ctx.font = 'bold 13px "Cascadia Mono", monospace'; ctx.fillStyle = '#0D1117';
      ctx.fillText('> L 127.0.0.1:5432 -> 127.0.0.1:5432', 64, 192);

      if (added) {
        ctx.font = '13px "Cascadia Mono", monospace'; ctx.fillStyle = '#D787FF';
        ctx.fillText('  D 127.0.0.1:1080 (SOCKS5)', 64, 224);
        roundRect(60, 250, 492, 46, 8, '#142C1F', '#87D787', 1.2);
        ctx.font = 'bold 13px "Cascadia Mono", monospace'; ctx.fillStyle = '#87D787';
        ctx.fillText('✓ Added dynamic SOCKS5 tunnel on prod-db-01!', 74, 278);
      } else {
        ctx.font = '13px "Cascadia Mono", monospace'; ctx.fillStyle = '#808080';
        ctx.fillText('Type : [ ] local   [ ] remote   [x] dynamic', 64, 232);
        ctx.fillStyle = '#5FD7FF';
        ctx.fillText('Spec : 127.0.0.1:1080▌  (SOCKS5 loopback)', 64, 260);
      }

      ctx.font = '12px "Cascadia Mono", monospace'; ctx.fillStyle = '#808080';
      ctx.fillText('CLI equivalent:', 60, 346);
      ctx.fillStyle = '#5FD7FF';
      ctx.fillText('sshm prod-db-01 -L 5432:127.0.0.1:5432 -D 1080', 60, 370);
      ctx.fillStyle = '#808080';
      ctx.fillText('[a] Add Tunnel  [d] Remove  [Tab] Type  [Esc] Back', 60, 446);

      // Right mini illustration
      roundRect(610, 82, 330, 408, 12, '#161B22', '#5FD7FF', 1.5);
      ctx.save();
      ctx.beginPath(); ctx.roundRect(624, 96, 302, 170, 8); ctx.clip();
      ctx.drawImage(imgs.network, 624, 96, 302, 170);
      ctx.restore();
      roundRect(624, 96, 302, 170, 8, null, '#263040', 1);

      ctx.font = 'bold 14.5px "Segoe UI", sans-serif'; ctx.fillStyle = '#5FD7FF';
      ctx.fillText('Encrypted Tunnel Flow', 626, 294);
      const tNotes = [
        '• -L Local: Connect DBeaver to',
        '  127.0.0.1:5432 securely.',
        '• -D SOCKS5: Route browser or',
        '  curl into private VPCs.',
        '• -R Remote: Expose local dev',
        '  server port to remote host.',
        '• 10s SOCKS5 handshake timeout',
        '  + non-loopback bind warning.',
      ];
      ctx.font = '12.5px "Segoe UI", sans-serif';
      tNotes.forEach((n, i) => { ctx.fillStyle = '#C9D1D9'; ctx.fillText(n, 626, 320 + i * 20); });

      drawFooter('Demo 3/3 · Persistent tunnels auto-start when connecting via TUI or CLI');
    }, 1500);
  }

  // Scene 3: Step 2/4 Parallel Batch Exec
  push(() => {
    ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · POWER OPS', 2, 4, 'Execute Commands Across Entire @group in Parallel', '--parallel');

    // Top fan-out visual bar
    const nodes = [
      { name: 'prod-api-01', ip: '10.0.1.12', st: '✓ 0.14s', col: '#87D787' },
      { name: 'prod-api-02', ip: '10.0.1.13', st: '✓ 0.19s', col: '#87D787' },
      { name: 'prod-db-01',  ip: '10.0.1.20', st: '✓ 0.24s', col: '#87D787' },
    ];
    // Draw connector bus cleanly behind node cards first
    ctx.strokeStyle = '#87D787'; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.moveTo(234, 122); ctx.lineTo(750, 122); ctx.stroke();

    roundRect(24, 80, 210, 84, 10, '#161B22', '#5FD7FF', 1.5);
    ctx.font = 'bold 13.5px "Cascadia Mono", monospace'; ctx.fillStyle = '#5FD7FF';
    ctx.fillText('sshm exec @prod', 44, 114);
    ctx.font = '12px "Segoe UI", sans-serif'; ctx.fillStyle = '#8B949E';
    ctx.fillText('Mutex-safe concurrent fan-out', 44, 140);

    nodes.forEach((n, i) => {
      const nx = 290 + i * 222;
      roundRect(nx, 80, 204, 84, 10, '#161B22', n.col, 1.5);
      ctx.font = 'bold 13px "Cascadia Mono", monospace'; ctx.fillStyle = '#F0F6FC';
      ctx.fillText(n.name, nx + 14, 108);
      ctx.font = '12px "Cascadia Mono", monospace'; ctx.fillStyle = '#8B949E';
      ctx.fillText(n.ip, nx + 14, 128);
      ctx.fillStyle = n.col; ctx.fillText(n.st + ' · exit 0', nx + 14, 148);
    });

    drawTerminalFrame(24, 182, 912, 308, 'PowerShell 7 — Parallel Fleet Operations', 'dark');
    const out = [
      ['#5FD7FF', 'PS C:\\> sshm exec @prod "uptime && df -h / | tail -n 1" --parallel'],
      ['#87D787', '[prod-api-01] 16:45:02 up 42 days, 03:11, load average: 0.18, 0.22, 0.19'],
      ['#D0D0D0', '[prod-api-01] /dev/nvme0n1p1  80G   24G   56G  30% /'],
      ['#87D787', '[prod-api-02] 16:45:02 up 42 days, 03:10, load average: 0.14, 0.19, 0.17'],
      ['#D0D0D0', '[prod-api-02] /dev/nvme0n1p1  80G   23G   57G  29% /'],
      ['#87D787', '[prod-db-01]  16:45:02 up 88 days, 14:22, load average: 0.42, 0.39, 0.35'],
      ['#D0D0D0', '[prod-db-01]  /dev/nvme0n1p1 500G  182G  318G  37% /'],
      ['#AF87FF', '✓ Completed 3/3 hosts in 248ms (0 errors · atomic LastConnected updated)'],
    ];
    ctx.font = '12.5px "Cascadia Mono", monospace';
    out.forEach(([c, l], i) => { ctx.fillStyle = c; ctx.fillText(l, 42, 238 + i * 28); });
    drawFooter('Demo 3/3 · Race-free parallel execution verified under "go test -race ./..."');
  }, 2200);

  // Scene 4: Step 3/4 SFTP Copy progress
  for (const pct of [40, 100]) {
    push(() => {
      ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
      drawStepHeader('HOW-TO · POWER OPS', 3, 4, 'Upload & Download Files via Built-In Pure-Go SFTP', 'sshm cp');
      drawTerminalFrame(24, 86, 912, 398, 'PowerShell 7 — Native SFTP Transfer (Zero scp.exe / WinSCP)', 'dark');
      ctx.font = '13px "Cascadia Mono", monospace';
      ctx.fillStyle = '#808080'; ctx.fillText('# Upload release artifact over pure-Go SFTP through ProxyJump', 42, 144);
      ctx.fillStyle = '#5FD7FF'; ctx.fillText('PS C:\\> sshm cp .\\dist\\release-v2.4.tar.gz prod-api-01:/opt/releases/', 42, 174);

      // Visual progress bar inside terminal
      roundRect(42, 202, 620, 26, 6, '#161B22', '#5FD7FF', 1.2);
      roundRect(44, 204, Math.round(616 * (pct / 100)), 22, 5, pct === 100 ? '#87D787' : '#5FD7FF', null);
      ctx.font = 'bold 13px "Cascadia Mono", monospace'; ctx.fillStyle = '#F0F6FC';
      ctx.fillText(pct + '%  (' + (pct === 100 ? '14.2 MB / 14.2 MB · DONE' : '5.7 MB / 14.2 MB · 6.4 MB/s') + ')', 678, 220);

      if (pct === 100) {
        ctx.fillStyle = '#87D787'; ctx.fillText('✓ Uploaded release-v2.4.tar.gz -> prod-api-01:/opt/releases/release-v2.4.tar.gz', 42, 262);
        ctx.fillStyle = '#808080'; ctx.fillText('# Pull production logs back to Windows:', 42, 314);
        ctx.fillStyle = '#5FD7FF'; ctx.fillText('PS C:\\> sshm cp prod-api-01:/var/log/app/error.log .\\logs\\error.log', 42, 344);
        ctx.fillStyle = '#87D787'; ctx.fillText('✓ Downloaded 428 KB cleanly over SFTP', 42, 374);
      }
      drawFooter('Demo 3/3 · Built-in github.com/pkg/sftp works seamlessly through ProxyJump bastions');
    }, pct === 100 ? 1800 : 750);
  }

  // Scene 5: Step 4/4 Encrypted Backup & Import
  push(() => {
    ctx.fillStyle = '#0D1117'; ctx.fillRect(0, 0, W, H);
    drawStepHeader('HOW-TO · POWER OPS', 4, 4, 'Encrypted Workstation Migration & PuTTY/OpenSSH Import', 'sshm export');
    drawTerminalFrame(24, 86, 912, 398, 'PowerShell 7 — Import & Encrypted .sshm.enc Backup', 'dark');
    const lines = [
      ['#808080', '# Import existing PuTTY Windows Registry sessions & ~/.ssh/config'],
      ['#5FD7FF', 'PS C:\\> sshm import --putty --group legacy'],
      ['#87D787', '✓ Imported 18 hosts from HKCU\\Software\\SimonTatham\\PuTTY\\Sessions'],
      ['#5FD7FF', 'PS C:\\> sshm import --ssh-config'],
      ['#87D787', '✓ Imported 12 hosts from ~/.ssh/config'],
      ['#D0D0D0', ''],
      ['#808080', '# Create Argon2id + AES-256-GCM encrypted backup for a new Windows laptop'],
      ['#5FD7FF', 'PS C:\\> sshm export .\\workstation-backup.sshm.enc'],
      ['#FFAF5F', 'Backup passphrase: ****************'],
      ['#87D787', '✓ Exported 36 hosts + 4 groups + credentials (Argon2id + AES-256-GCM)'],
      ['#808080', '# Restore anytime: sshm import --backup .\\workstation-backup.sshm.enc'],
    ];
    ctx.font = '13px "Cascadia Mono", monospace';
    lines.forEach(([c, l], i) => { ctx.fillStyle = c; ctx.fillText(l, 42, 144 + i * 28); });
    drawFooter('Demo 3/3 Complete · Explore all 6 playbooks in docs/USE_CASES.md');
  }, 2200);

  return frames;
}

async function recordDemo(name, frames) {
  const stream = canvas.captureStream(15);
  const mime = MediaRecorder.isTypeSupported('video/webm;codecs=vp8')
    ? 'video/webm;codecs=vp8'
    : 'video/webm';
  const rec = new MediaRecorder(stream, { mimeType: mime, videoBitsPerSecond: 1600000 });
  const webmChunks = [];
  rec.ondataavailable = (e) => { if (e.data && e.data.size > 0) webmChunks.push(e.data); };
  const stopped = new Promise(r => { rec.onstop = r; });
  rec.start();

  const gifMeta = [];
  for (let i = 0; i < frames.length; i++) {
    const f = frames[i];
    f.drawFn();
    // Grab RGBA pixels for GIF frame
    const imgData = ctx.getImageData(0, 0, W, H);
    const resp = await fetch('/upload-frame?demo=' + encodeURIComponent(name) + '&idx=' + i + '&delay=' + f.delayMs, {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body: imgData.data.buffer,
    });
    if (!resp.ok) throw new Error('Frame upload failed');
    gifMeta.push(i);
    // Wait real dwell time so WebM video plays at smooth readable pace
    await new Promise(r => setTimeout(r, f.delayMs));
  }

  rec.stop();
  await stopped;

  const blob = new Blob(webmChunks, { type: 'video/webm' });
  const ab = await blob.arrayBuffer();
  const wResp = await fetch('/upload-webm?demo=' + encodeURIComponent(name), {
    method: 'POST',
    headers: { 'Content-Type': 'video/webm' },
    body: ab,
  });
  if (!wResp.ok) throw new Error('WebM upload failed');
}

(async function main() {
  try {
    const imgs = {
      hero: await loadImage('/illust/hero-banner.png'),
      vault: await loadImage('/illust/security-credential-vault.png'),
      network: await loadImage('/illust/network-proxyjump-tunnels.png'),
    };
    await recordDemo('demo-tui-agile-workflow', buildDemo1Frames(imgs));
    await recordDemo('demo-add-host-wincred-connect', buildDemo2Frames(imgs));
    await recordDemo('demo-tunnels-batch-sftp', buildDemo3Frames(imgs));
    await fetch('/done', { method: 'POST' });
  } catch (err) {
    await fetch('/error', { method: 'POST', body: String(err && err.stack || err) });
  }
})();
</script>
</body>
</html>`;
}

// ---------------------------------------------------------------------------
// Main Runner
// ---------------------------------------------------------------------------
async function main() {
  const browser = findBrowser();
  console.log(`Using browser: ${browser}`);

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "sshm-media-"));
  const profileDir = path.join(tmpDir, "profile");

  // 1. Render all 6 High-DPI TUI Mockups (.png)
  console.log("\n[1/2] Rendering 6 High-DPI TUI Mockup PNGs...");
  for (const m of MOCKUPS) {
    const htmlPath = path.join(tmpDir, m.name + ".html");
    const outPath = path.join(MOCKUP_DIR, m.name);
    fs.writeFileSync(htmlPath, m.html, "utf8");
    const fileUrl = "file:///" + htmlPath.replace(/\\/g, "/");
    const res = spawnSync(
      browser,
      [
        "--headless=new",
        "--disable-gpu",
        "--no-sandbox",
        "--hide-scrollbars",
        "--force-device-scale-factor=2",
        "--window-size=1200,700",
        `--user-data-dir=${profileDir}`,
        `--screenshot=${outPath}`,
        fileUrl,
      ],
      { stdio: "pipe", timeout: 30000 }
    );
    if (res.status !== 0 || !fs.existsSync(outPath)) {
      throw new Error(`Failed to render ${m.name}: ${res.stderr?.toString()}`);
    }
    console.log(`  ✓ ${m.name} (${fs.statSync(outPath).size} bytes)`);
  }

  // 2. Render all 3 Illustrated How-To Demo Videos (.gif + .webm)
  console.log("\n[2/2] Rendering 3 Illustrated How-To Demo Videos (.gif + .webm)...");
  const demoFrames = new Map(); // demoName -> [{ idx, delayMs, indexed }]
  const STUDIO_HTML = buildVideoStudioHtml();

  await new Promise((resolve, reject) => {
    let browserProc = null;
    const server = http.createServer((req, res) => {
      const url = new URL(req.url, "http://127.0.0.1");
      if (url.pathname === "/") {
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(STUDIO_HTML);
        return;
      }
      if (url.pathname.startsWith("/illust/")) {
        const fname = path.basename(url.pathname);
        const fpath = path.join(ILLUST_DIR, fname);
        if (fs.existsSync(fpath)) {
          res.writeHead(200, { "Content-Type": "image/png" });
          fs.createReadStream(fpath).pipe(res);
          return;
        }
        res.writeHead(404);
        res.end("Not found");
        return;
      }
      if (url.pathname === "/upload-frame" && req.method === "POST") {
        const demo = url.searchParams.get("demo");
        const idx = Number(url.searchParams.get("idx"));
        const delayMs = Number(url.searchParams.get("delay"));
        const bufs = [];
        req.on("data", (c) => bufs.push(c));
        req.on("end", () => {
          const rgba = Buffer.concat(bufs);
          const indexed = quantizeRGBA(rgba, 960 * 540);
          if (!demoFrames.has(demo)) demoFrames.set(demo, []);
          demoFrames.get(demo).push({ idx, delayMs, indexed });
          res.writeHead(200);
          res.end("ok");
        });
        return;
      }
      if (url.pathname === "/upload-webm" && req.method === "POST") {
        const demo = url.searchParams.get("demo");
        const bufs = [];
        req.on("data", (c) => bufs.push(c));
        req.on("end", () => {
          const webmBuf = Buffer.concat(bufs);
          const webmPath = path.join(VIDEO_DIR, `${demo}.webm`);
          fs.writeFileSync(webmPath, webmBuf);
          console.log(`  ✓ ${demo}.webm (${webmBuf.length} bytes)`);

          // Encode corresponding animated GIF89a
          const frames = (demoFrames.get(demo) || []).sort((a, b) => a.idx - b.idx);
          const gifBuf = encodeGIF(960, 540, frames);
          const gifPath = path.join(VIDEO_DIR, `${demo}.gif`);
          fs.writeFileSync(gifPath, gifBuf);
          console.log(`  ✓ ${demo}.gif (${gifBuf.length} bytes, ${frames.length} frames)`);

          res.writeHead(200);
          res.end("ok");
        });
        return;
      }
      if (url.pathname === "/done") {
        res.writeHead(200);
        res.end("bye");
        if (browserProc) browserProc.kill();
        server.close(() => resolve());
        return;
      }
      if (url.pathname === "/error") {
        const bufs = [];
        req.on("data", (c) => bufs.push(c));
        req.on("end", () => {
          res.writeHead(200);
          res.end("err");
          if (browserProc) browserProc.kill();
          server.close(() => reject(new Error(Buffer.concat(bufs).toString())));
        });
        return;
      }
      res.writeHead(404);
      res.end();
    });

    server.listen(0, "127.0.0.1", () => {
      const port = server.address().port;
      const studioUrl = `http://127.0.0.1:${port}/`;
      const vidProfile = path.join(tmpDir, "vid-profile");
      browserProc = spawn(
        browser,
        [
          "--headless=new",
          "--no-sandbox",
          "--autoplay-policy=no-user-gesture-required",
          `--user-data-dir=${vidProfile}`,
          "--window-size=1000,600",
          studioUrl,
        ],
        { stdio: "pipe" }
      );
      browserProc.on("error", (err) => {
        server.close();
        reject(err);
      });
    });

    setTimeout(() => {
      if (browserProc) browserProc.kill();
      server.close();
      reject(new Error("Timed out waiting for video renderer"));
    }, 120000).unref();
  });

  fs.rmSync(tmpDir, { recursive: true, force: true });
  console.log("\nAll mockups (.png) and illustrated how-to videos (.gif + .webm) rendered cleanly!");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
