#!/usr/bin/env node
// One-time codemod for #198: rewrite relative imports in src/ to alias
// form. Run from frontend/: `node scripts/codemod-aliases.mjs`.
//
// Rules:
//   - Walk all *.ts / *.tsx under src/.
//   - For each `from '...'` / `from "..."` / `import('...')` whose
//     target starts with `../`, resolve to an absolute path under src/.
//   - If the target lives under one of the aliased layers (app, shared,
//     features, engine, theme, storage), rewrite to `@<layer>/<rest>`.
//   - Targets under legacy dirs (pages, components, hooks, services)
//     stay relative — they aren't aliased so the rule can't enforce
//     them, and emphasising their target-for-migration status is
//     intentional.
//   - Sibling imports (`./X`) stay as is — within-folder relatives are
//     allowed by `import/no-relative-parent-imports`.

import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { resolve, dirname, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(here, '..', 'src');

const ALIASED_LAYERS = new Set([
  // BR layers (target homes).
  'app',
  'shared',
  'features',
  'engine',
  'theme',
  'storage',
  // Legacy dirs (target-for-migration; aliased only so the lint rule
  // can enforce alias-only imports during the #176 transition).
  'pages',
  'components',
  'hooks',
  'services',
]);

function listTsFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = resolve(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...listTsFiles(full));
    } else if (entry.isFile() && /\.(ts|tsx)$/.test(entry.name)) {
      out.push(full);
    }
  }
  return out;
}

function tryAlias(fromFile, importPath) {
  if (!importPath.startsWith('../')) return null;
  const absTarget = resolve(dirname(fromFile), importPath);
  const rel = relative(srcRoot, absTarget);
  // If the target escapes src/, leave it alone.
  if (rel.startsWith('..') || rel.startsWith(sep)) return null;
  const [first, ...rest] = rel.split(sep);
  if (!ALIASED_LAYERS.has(first)) return null;
  return `@${first}/${rest.join('/')}`;
}

const files = listTsFiles(srcRoot);
let changedFiles = 0;
let changedImports = 0;

// Matches the literal in:
//   import X from '...'
//   import X from "..."
//   import('...')
//   import("...")
//   export ... from '...'
//   export ... from "..."
const IMPORT_RE = /(from\s+|import\s*\()(['"])([^'"]+)\2/g;

for (const file of files) {
  const original = readFileSync(file, 'utf8');
  let touched = 0;
  const next = original.replace(IMPORT_RE, (match, prefix, quote, spec) => {
    const aliased = tryAlias(file, spec);
    if (aliased === null) return match;
    touched += 1;
    return `${prefix}${quote}${aliased}${quote}`;
  });
  if (touched > 0) {
    writeFileSync(file, next);
    changedFiles += 1;
    changedImports += touched;
    console.log(`  ${relative(srcRoot, file)}: ${touched} import(s)`);
  }
}

console.log(`\nRewrote ${changedImports} import(s) across ${changedFiles} file(s).`);
