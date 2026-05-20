#!/usr/bin/env node
// One-time codemod for #198: rewrite relative imports in src/ to alias
// form. Run from frontend/: `node scripts/codemod-aliases.mjs`.
//
// Rules:
//   - Walk all *.ts / *.tsx under src/.
//   - For each `from '...'` / `from "..."` / `import('...')` /
//     `vi.mock('...')` / `vi.doMock('...')` / `vi.unmock('...')` /
//     `vi.importActual('...')` / `vi.importMock('...')` whose target
//     starts with `../`, resolve to an absolute path under src/.
//   - If the target lives under one of the aliased layers — BR
//     (app, shared, features, engine, theme, storage) OR transitional
//     legacy (pages, components, hooks, services) — rewrite to
//     `@<layer>/<rest>`. The legacy aliases exist so the
//     `no-restricted-imports` ESLint rule can enforce alias-only
//     imports without per-file exemptions during the #176 transition.
//   - Sibling imports (`./X`) stay as-is — within-folder relatives are
//     allowed by `no-restricted-imports`.
//   - The script is idempotent: re-running it on the converted tree
//     produces zero changes (the rule guards on `../` prefix).

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
  // Transitional legacy-dir aliases. Drop as each migrates per #176.
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
//   vi.mock('...')   /   vi.mock("...")   /   vi.doMock(...)
//   vi.unmock('...') / vi.importActual(...) / vi.importMock(...)
const IMPORT_RE =
  /(from\s+|import\s*\(|vi\.(?:mock|doMock|unmock|importActual|importMock)\s*\()(['"])([^'"]+)\2/g;

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
