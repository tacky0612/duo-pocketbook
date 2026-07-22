#!/usr/bin/env node
// docs/ 配下の Markdown に含まれる ```mermaid ブロックを抽出し、
// @mermaid-js/mermaid-cli (mmdc) で構文検証する。
// 崩れた Mermaid 記法があれば非ゼロ終了する。
//
// 使い方: node scripts/validate-mermaid.mjs [対象ディレクトリ(既定: docs)]

import { readdirSync, readFileSync, writeFileSync, mkdtempSync, rmSync } from "node:fs";
import { join, extname } from "node:path";
import { tmpdir } from "node:os";
import { execFileSync } from "node:child_process";

const targetDir = process.argv[2] ?? "docs";

/** 指定ディレクトリ配下の .md を再帰的に集める */
function markdownFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...markdownFiles(full));
    else if (extname(entry.name) === ".md") out.push(full);
  }
  return out;
}

/** Markdown から ```mermaid ... ``` ブロックを抽出する（開始行番号付き） */
function extractMermaidBlocks(content) {
  const lines = content.split("\n");
  const blocks = [];
  let current = null;
  lines.forEach((line, idx) => {
    if (current === null) {
      if (/^```mermaid\s*$/.test(line.trim())) current = { startLine: idx + 1, body: [] };
    } else if (line.trim() === "```") {
      blocks.push({ startLine: current.startLine, code: current.body.join("\n") });
      current = null;
    } else {
      current.body.push(line);
    }
  });
  return blocks;
}

const files = markdownFiles(targetDir);
const work = mkdtempSync(join(tmpdir(), "mermaid-validate-"));
let total = 0;
const failures = [];

try {
  for (const file of files) {
    const blocks = extractMermaidBlocks(readFileSync(file, "utf8"));
    blocks.forEach((block, i) => {
      total += 1;
      const inFile = join(work, `block-${i}.mmd`);
      const outFile = join(work, `block-${i}.svg`);
      writeFileSync(inFile, block.code);
      try {
        execFileSync(
          "npx",
          ["-y", "@mermaid-js/mermaid-cli@11", "-i", inFile, "-o", outFile, "-q"],
          { stdio: "pipe" }
        );
        console.log(`  ok  ${file}:${block.startLine}`);
      } catch (err) {
        const detail = (err.stderr?.toString() || err.message || "").trim();
        failures.push({ file, line: block.startLine, detail });
        console.error(`FAIL  ${file}:${block.startLine}`);
      }
    });
  }
} finally {
  rmSync(work, { recursive: true, force: true });
}

console.log(`\nMermaidブロック ${total}件を検証しました。`);
if (failures.length > 0) {
  console.error(`\n${failures.length}件の構文エラー:`);
  for (const f of failures) console.error(`\n● ${f.file}:${f.line}\n${f.detail}`);
  process.exit(1);
}
if (total === 0) console.log("（検証対象の Mermaid ブロックはありませんでした）");
console.log("すべての Mermaid 記法は正しく解析できました。");
