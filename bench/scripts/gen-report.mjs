#!/usr/bin/env node
// Turn bench/results/*.json (hyperfine output) into:
//   1. a Markdown summary table + per-case sections
//   2. one horizontal-bar SVG chart per case (docs/public/bench/<slug>.svg)
// and splice all of it into the docs page between the BENCH:RESULTS markers.
//
// No external dependencies — the SVGs are plain strings.
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(SCRIPT_DIR, "..", "..");
const BENCH = resolve(REPO, "bench");
const RESULTS = resolve(BENCH, "results");
const PUBLIC_SVG = resolve(REPO, "docs", "public", "bench");
const DOCS_PAGE = resolve(
  REPO,
  "docs",
  "src",
  "content",
  "docs",
  "performance",
  "benchmarks.mdx"
);

const CASES = ["fib-recursive", "loop-sum", "prime-sieve", "sort", "matmul"];
const LANGS = [
  { key: "zeus", label: "Zeus", color: "#f59e0b" },
  { key: "go", label: "Go", color: "#00add8" },
  { key: "node", label: "Node.js", color: "#57a548" },
];

const fmtTime = (s) =>
  s < 1 ? `${(s * 1000).toFixed(0)} ms` : `${s.toFixed(2)} s`;
const fmtPM = (mean, sd) =>
  mean < 1
    ? `${(mean * 1000).toFixed(0)} ± ${(sd * 1000).toFixed(0)} ms`
    : `${mean.toFixed(2)} ± ${sd.toFixed(2)} s`;
const ratio = (a, b) => `${(a / b).toFixed(2)}×`;

function readJSON(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

// Map a hyperfine export ({results:[{command, mean, stddev, ...}]}) to {lang: stats}
function statsForCase(slug) {
  const data = readJSON(resolve(RESULTS, `${slug}.json`));
  const byName = {};
  for (const r of data.results) byName[r.command] = r;
  const out = {};
  for (const { key } of LANGS) {
    const r = byName[key];
    if (!r) throw new Error(`missing '${key}' result in ${slug}.json`);
    out[key] = { mean: r.mean, stddev: r.stddev ?? 0 };
  }
  return out;
}

function svgChart(meta, stats) {
  const W = 540;
  const rowH = 40;
  const top = 52;
  const left = 84;
  const barMax = 320;
  const bottom = 16;
  const H = top + LANGS.length * rowH + bottom;
  const maxMean = Math.max(...LANGS.map((l) => stats[l.key].mean));

  let rows = "";
  LANGS.forEach((l, i) => {
    const { mean } = stats[l.key];
    const y = top + i * rowH;
    const w = Math.max(3, (mean / maxMean) * barMax);
    rows += `
    <text x="${left - 12}" y="${y + 19}" text-anchor="end" font-size="14" fill="#94a3b8">${l.label}</text>
    <rect x="${left}" y="${y + 3}" width="${w.toFixed(1)}" height="24" rx="4" fill="${l.color}" />
    <text x="${(left + w + 8).toFixed(1)}" y="${y + 20}" font-size="13" fill="#94a3b8">${fmtTime(mean)}</text>`;
  });

  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" font-family="ui-sans-serif, system-ui, sans-serif" role="img" aria-label="${meta.name} benchmark: wall-clock time, lower is better">
    <text x="16" y="26" font-size="16" font-weight="600" fill="#cbd5e1">${meta.name}</text>
    <text x="16" y="44" font-size="12" fill="#64748b">mean wall-clock time — lower is better (${meta.param})</text>${rows}
  </svg>\n`;
}

function summaryTable(all) {
  const head =
    "| Benchmark | Zeus | Go | Node.js | Zeus vs Go | Zeus vs Node |\n" +
    "|-----------|------|----|---------|-----------|--------------|";
  const rows = CASES.map((slug) => {
    const { meta, stats } = all[slug];
    const z = stats.zeus,
      g = stats.go,
      n = stats.node;
    return `| ${meta.name} | ${fmtPM(z.mean, z.stddev)} | ${fmtPM(
      g.mean,
      g.stddev
    )} | ${fmtPM(n.mean, n.stddev)} | ${ratio(z.mean, g.mean)} | ${ratio(
      z.mean,
      n.mean
    )} |`;
  });
  return [head, ...rows].join("\n");
}

function caseSections(all) {
  return CASES.map((slug) => {
    const { meta } = all[slug];
    return `### ${meta.name}

${meta.description}

![${meta.name} benchmark chart](/bench/${slug}.svg)`;
  }).join("\n\n");
}

function main() {
  const all = {};
  for (const slug of CASES) {
    const meta = readJSON(resolve(BENCH, "cases", slug, "meta.json"));
    all[slug] = { meta, stats: statsForCase(slug) };
  }

  // Write SVG charts.
  for (const slug of CASES) {
    writeFileSync(resolve(PUBLIC_SVG, `${slug}.svg`), svgChart(all[slug].meta, all[slug].stats));
  }

  // Environment line.
  const env = existsSync(resolve(RESULTS, "env.json"))
    ? readJSON(resolve(RESULTS, "env.json"))
    : null;
  const hf = env ? String(env.hyperfine).replace(/^hyperfine\s+/, "") : "";
  const envLine = env
    ? `_Measured ${env.date} on ${env.cpu}, ${env.os}. Go ${env.go}, Node ${env.node}, hyperfine ${hf}. Zeus \`${env.zeus}\`._`
    : "";

  // Consolidated machine-readable summary (committed alongside the report).
  writeFileSync(
    resolve(RESULTS, "summary.json"),
    JSON.stringify(
      { env, cases: Object.fromEntries(CASES.map((s) => [s, all[s].stats])) },
      null,
      2
    ) + "\n"
  );

  const block = [
    envLine,
    "",
    "## Summary",
    "",
    "Wall-clock time per run — **lower is better**. `Zeus vs Go` / `Zeus vs Node` are Zeus's time divided by the other language's: below 1× means Zeus is faster, above 1× means slower.",
    "",
    summaryTable(all),
    "",
    "## Results by benchmark",
    "",
    caseSections(all),
  ].join("\n");

  const START = "{/* BENCH:RESULTS:START */}";
  const END = "{/* BENCH:RESULTS:END */}";
  const page = readFileSync(DOCS_PAGE, "utf8");
  const s = page.indexOf(START);
  const e = page.indexOf(END);
  if (s === -1 || e === -1) {
    throw new Error(`markers ${START} / ${END} not found in ${DOCS_PAGE}`);
  }
  const next =
    page.slice(0, s + START.length) + "\n\n" + block + "\n\n" + page.slice(e);
  writeFileSync(DOCS_PAGE, next);

  console.log("Wrote charts to docs/public/bench/, summary.json, and updated benchmarks.mdx");
}

main();
