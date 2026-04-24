#!/usr/bin/env node
// Verifies that github-slugger (the engine rehype-slug uses) produces the
// same output as Go's internal/render.Slugify for every vector in
// ../../internal/render/testdata/slug_vectors.json. Called from the
// frontend's prebuild hook; divergence breaks `npm run build`.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import GithubSlugger from "github-slugger";

const __dirname = dirname(fileURLToPath(import.meta.url));
const vectorsPath = resolve(
  __dirname,
  "../../internal/render/testdata/slug_vectors.json"
);

const vectors = JSON.parse(readFileSync(vectorsPath, "utf8"));

let failed = 0;
for (const { input, history, want } of vectors) {
  const slugger = new GithubSlugger();
  // Reconstruct prior state by calling slug() with each previously seen slug.
  // Each entry in history is a slug that was already output (e.g. "foo",
  // "foo-1"). Calling slugger.slug(h) correctly advances the internal
  // occurrences counter to match what rehype-slug would have accumulated.
  for (const h of history) slugger.slug(h);
  const got = slugger.slug(input);
  if (got !== want) {
    console.error(
      `✗ slug parity mismatch: slug(${JSON.stringify(input)}, history=${JSON.stringify(history)}) = ${JSON.stringify(got)}, want ${JSON.stringify(want)}`
    );
    failed++;
  }
}

if (failed > 0) {
  console.error(`\n${failed} slug vector(s) diverge between Go and github-slugger.`);
  console.error("Fix either internal/render/slug.go or testdata/slug_vectors.json so both agree.");
  process.exit(1);
}

console.log(`slug parity: ${vectors.length} vectors match github-slugger`);
