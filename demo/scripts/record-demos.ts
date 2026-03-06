/**
 * Record demo videos for each publisher using Playwright.
 *
 * Prerequisites:
 *   pnpm add -D playwright
 *   npx playwright install chromium
 *
 * Usage:
 *   npx tsx scripts/record-demos.ts [publisher...]
 *
 * Examples:
 *   npx tsx scripts/record-demos.ts              # record all publishers
 *   npx tsx scripts/record-demos.ts chai amp       # record specific ones
 *
 * The dev server must be running on localhost:6969 and the
 * ad-server backend must be running on localhost:8080 with an
 * Anthropic API key configured.
 */

import { chromium } from "playwright";
import path from "path";
import fs from "fs/promises";

const ALL_PUBLISHERS = [
  "chai",
  "amp",
  "luzia",
  "kindroid",
  "galenai",
  "autonomous",
  "sonia",
  "youlearn",
  "alice",
];

const BASE_URL = "http://localhost:6969";
const BACKEND_URL = "http://localhost:8080";
const OUTPUT_DIR = path.resolve(import.meta.dirname, "../recordings");
const TIMEOUT = 5 * 60 * 1000; // 5 minutes per publisher

async function preflight() {
  // Check frontend
  const frontResp = await fetch(BASE_URL).catch(() => null);
  if (!frontResp?.ok) throw new Error(`Frontend not running at ${BASE_URL}`);

  // Check backend health
  const healthResp = await fetch(`${BACKEND_URL}/health`).catch(() => null);
  if (!healthResp?.ok) throw new Error(`Backend not running at ${BACKEND_URL}`);

  // Check chat proxy (needs Anthropic key)
  const chatResp = await fetch(`${BACKEND_URL}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages: [{ role: "user", content: "hi" }] }),
  }).catch(() => null);
  if (chatResp?.status === 503) {
    throw new Error("Chat proxy unavailable — start backend with ANTHROPIC_API_KEY");
  }

  // Check advertisers are seeded
  const posResp = await fetch(`${BACKEND_URL}/positions`);
  const positions = await posResp.json();
  if (!Array.isArray(positions) || positions.length < 10) {
    throw new Error(
      `Only ${positions?.length ?? 0} advertisers in DB — run: go run ./cmd/server/ --seed`
    );
  }

  console.log(`Preflight OK: ${positions.length} advertisers, chat proxy working`);
}

async function record(browser: Awaited<ReturnType<typeof chromium.launch>>, id: string) {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    recordVideo: {
      dir: OUTPUT_DIR,
      size: { width: 1280, height: 720 },
    },
  });

  const page = await context.newPage();
  await page.goto(`${BASE_URL}?publisher=${id}&replay=true`);

  // Wait for replay to finish
  await page.waitForSelector("[data-replay-done]", { timeout: TIMEOUT });
  await page.waitForTimeout(2000);

  const dest = path.join(OUTPUT_DIR, `demo-${id}.webm`);
  await page.video()?.saveAs(dest);
  await context.close();

  const stat = await fs.stat(dest);
  const sizeMB = (stat.size / 1024 / 1024).toFixed(1);
  console.log(`  Saved: demo-${id}.webm (${sizeMB} MB)`);
}

async function main() {
  // Parse CLI args for specific publishers
  const args = process.argv.slice(2);
  const publishers = args.length > 0
    ? args.filter((a) => {
        if (!ALL_PUBLISHERS.includes(a)) {
          console.error(`Unknown publisher: ${a}. Valid: ${ALL_PUBLISHERS.join(", ")}`);
          process.exit(1);
        }
        return true;
      })
    : ALL_PUBLISHERS;

  await preflight();
  await fs.mkdir(OUTPUT_DIR, { recursive: true });

  const browser = await chromium.launch();

  for (const id of publishers) {
    console.log(`Recording ${id}...`);
    try {
      await record(browser, id);
    } catch (err) {
      console.error(`  FAILED: ${err instanceof Error ? err.message : err}`);
    }
  }

  await browser.close();
  console.log(`\nDone! ${publishers.length} recordings in ${OUTPUT_DIR}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
