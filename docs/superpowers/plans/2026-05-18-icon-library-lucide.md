# Icon Library Adoption (Lucide) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt `lucide-react` as the project's icon library, ship a tiny `<Icon>` wrapper that pins brand defaults (size 20, strokeWidth 1.5), and migrate the three existing chrome glyph sites (back arrow, dark-mode toggle, Install button) to use it.

**Architecture:** Per-site direct imports from `lucide-react`, wrapped through `frontend/src/shared/components/Icon.tsx`. No barrel re-export, no `IconContext` provider. The wrapper is the seam if the library is ever swapped.

**Tech Stack:** React 19, TypeScript strict, Vite, Vitest (unit), Playwright (e2e), `lucide-react@^1.16.0` (ISC license).

**Spec:** `docs/superpowers/specs/2026-05-18-icon-library-design.md`
**Issue:** [#180](https://github.com/lesteenman/reign-game/issues/180) (umbrella: [#176](https://github.com/lesteenman/reign-game/issues/176))
**Branch:** `feat/180-icon-library-lucide`

---

## File map

**Created:**
- `frontend/src/shared/components/Icon.tsx` — wrapper component
- `frontend/src/shared/components/Icon.test.tsx` — unit tests for wrapper

**Modified:**
- `frontend/package.json` — add `lucide-react` to `dependencies`
- `frontend/package-lock.json` — npm-managed lockfile update
- `frontend/src/components/common/PageShell.tsx` — back arrow + dark-mode toggle adopt `<Icon>`
- `frontend/src/components/common/InstallButton.tsx` — adopt `<Icon as={Download} />` alongside text label
- `frontend/CLAUDE.md` — append "Icons" convention under UI Primitives

**No changes expected (verified during context exploration):**
- `frontend/src/components/common/PageShell.test.tsx` — asserts via `data-testid` and `aria-label`, not on glyph characters
- `frontend/src/components/common/InstallButton.test.tsx` — asserts `toHaveTextContent(/install/i)` which still passes (text label retained)
- `frontend/playwright/**/*.spec.ts` — no glyph-string matches; buttons identified by `data-testid`

Task 5 verifies these no-change assumptions by running the full test suite.

---

## Task 1: Install lucide-react

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json` (npm-managed)

- [ ] **Step 1: Add the dependency**

Run from the repo root:

```bash
cd frontend && npm install lucide-react@^1.16.0 && cd ..
```

Expected: package.json gains `"lucide-react": "^1.16.0"` under `dependencies`. package-lock.json updates accordingly. No peer-dependency warnings (lucide-react 1.16.0 accepts React 19).

- [ ] **Step 2: Verify the install**

```bash
cd frontend && node -e "import('lucide-react').then(m => console.log('ArrowLeft:', !!m.ArrowLeft, 'Sun:', !!m.Sun, 'Moon:', !!m.Moon, 'Download:', !!m.Download))" && cd ..
```

Expected: `ArrowLeft: true Sun: true Moon: true Download: true`

This confirms all four icons the migration uses exist as named exports in the installed version. If any prints `false`, stop and check the lucide-react CHANGELOG for the renamed export.

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "deps(frontend): add lucide-react@^1.16.0 (#180)"
```

---

## Task 2: Create the `<Icon>` wrapper (TDD)

**Files:**
- Create: `frontend/src/shared/components/Icon.test.tsx`
- Create: `frontend/src/shared/components/Icon.tsx`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/shared/components/Icon.test.tsx`:

```tsx
import { describe, it, expect, afterEach } from 'vitest';
import { ArrowLeft } from 'lucide-react';
import { render, screen, cleanup } from '../../test-utils';
import { Icon } from './Icon';

afterEach(() => {
  cleanup();
});

describe('Icon', () => {
  it('renders the lucide icon passed via the `as` prop', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-under-test" />);

    // Assert
    const svg = screen.getByTestId('icon-under-test');
    expect(svg).toBeInTheDocument();
    expect(svg.tagName.toLowerCase()).toBe('svg');
  });

  it('applies brand defaults: size 20, strokeWidth 1.5', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-defaults" />);

    // Assert
    const svg = screen.getByTestId('icon-defaults');
    expect(svg).toHaveAttribute('width', '20');
    expect(svg).toHaveAttribute('height', '20');
    expect(svg).toHaveAttribute('stroke-width', '1.5');
  });

  it('marks the icon aria-hidden by default (icons sit inside labeled buttons)', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} data-testid="icon-aria" />);

    // Assert
    const svg = screen.getByTestId('icon-aria');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('allows per-site override of size and strokeWidth', () => {
    // Arrange & Act
    render(<Icon as={ArrowLeft} size={32} strokeWidth={2} data-testid="icon-override" />);

    // Assert
    const svg = screen.getByTestId('icon-override');
    expect(svg).toHaveAttribute('width', '32');
    expect(svg).toHaveAttribute('height', '32');
    expect(svg).toHaveAttribute('stroke-width', '2');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/shared/components/Icon.test.tsx
```

Expected: All four tests FAIL with a module-resolution error like `Failed to resolve import "./Icon"` (the component doesn't exist yet).

- [ ] **Step 3: Write the minimal implementation**

Create `frontend/src/shared/components/Icon.tsx`:

```tsx
import type { LucideIcon, LucideProps } from 'lucide-react';

export interface IconProps extends Omit<LucideProps, 'ref'> {
  as: LucideIcon;
}

/**
 * Brand-default Lucide wrapper. Pins size 20 and strokeWidth 1.5 per
 * BRAND_GUIDELINES.md (Tab bar icons section). `aria-hidden` defaults
 * to true because every current adoption site is an icon inside a
 * `<button>` whose `aria-label` carries the accessible name.
 */
export function Icon({
  as: Component,
  size = 20,
  strokeWidth = 1.5,
  'aria-hidden': ariaHidden = true,
  ...rest
}: IconProps) {
  return (
    <Component
      size={size}
      strokeWidth={strokeWidth}
      aria-hidden={ariaHidden}
      {...rest}
    />
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/shared/components/Icon.test.tsx
```

Expected: All four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/shared/components/Icon.tsx frontend/src/shared/components/Icon.test.tsx
git commit -m "feat(frontend): add shared/components/Icon wrapper for Lucide (#180)"
```

---

## Task 3: Migrate the back arrow in PageShell

**Files:**
- Modify: `frontend/src/components/common/PageShell.tsx`

- [ ] **Step 1: Add the lucide-react import**

Edit `frontend/src/components/common/PageShell.tsx`. Add at the top, alongside existing imports:

```tsx
import { ArrowLeft, Sun, Moon } from 'lucide-react';
import { Icon } from '../../shared/components/Icon';
```

(Bringing in `Sun` and `Moon` here too so Task 4 doesn't repeat the edit; if you prefer, only `ArrowLeft` in this task and add the others in Task 4.)

- [ ] **Step 2: Replace the back-button glyph**

Find this block (around lines 105-114):

```tsx
            {'←'}
```

Replace with:

```tsx
            <Icon as={ArrowLeft} />
```

While editing, remove the now-unnecessary `fontSize: '1.25rem'` rule from the back button's style object — the SVG sizes itself. Keep all other style rules (padding, lineHeight, minWidth, minHeight, etc.).

- [ ] **Step 3: Run existing PageShell tests to verify no regression**

```bash
cd frontend && npx vitest run src/components/common/PageShell.test.tsx
```

Expected: All tests PASS. (They assert via `data-testid="back-button"` and the back button's `aria-label`, neither of which changed.)

If any test fails because it asserted on the literal `←` character, update that single assertion to `expect(screen.getByTestId('back-button').querySelector('svg')).toBeInTheDocument()` and re-run. (Pre-flight grep suggested this won't happen; this step covers the contingency.)

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/common/PageShell.tsx
git commit -m "feat(frontend): back arrow uses Lucide ArrowLeft via Icon wrapper (#180)"
```

---

## Task 4: Migrate the dark-mode toggle in PageShell

**Files:**
- Modify: `frontend/src/components/common/PageShell.tsx`

- [ ] **Step 1: Replace the dark-mode toggle glyph**

The `Sun` and `Moon` imports were added in Task 3 Step 1 already. If they were skipped, add them now:

```tsx
import { ArrowLeft, Sun, Moon } from 'lucide-react';
```

Find this block (around line 153):

```tsx
            {isDark ? '☀' : '☾'}
```

Replace with:

```tsx
            <Icon as={isDark ? Sun : Moon} />
```

While editing, remove the now-unnecessary `fontSize: '1.25rem'` rule from the dark-mode-toggle's style object.

- [ ] **Step 2: Run existing PageShell tests to verify no regression**

```bash
cd frontend && npx vitest run src/components/common/PageShell.test.tsx
```

Expected: All tests PASS. (Asserts via `data-testid="dark-mode-toggle"` and dynamic `aria-label="Switch to light/dark mode"` — neither changed.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/common/PageShell.tsx
git commit -m "feat(frontend): dark-mode toggle uses Lucide Sun/Moon via Icon wrapper (#180)"
```

---

## Task 5: Migrate the InstallButton

**Files:**
- Modify: `frontend/src/components/common/InstallButton.tsx`

- [ ] **Step 1: Add the icon to the Install button**

Edit `frontend/src/components/common/InstallButton.tsx`. Add the imports:

```tsx
import { Download } from 'lucide-react';
import { Icon } from '../../shared/components/Icon';
```

Find the button body:

```tsx
    <button
      type="button"
      data-testid="install-button"
      onClick={() => { void promptInstall(); }}
      aria-label="Install Reign as an app"
      title="Install Reign"
      style={buttonStyle}
    >
      Install
    </button>
```

Replace the body content with the icon + label, separated by a small gap:

```tsx
    <button
      type="button"
      data-testid="install-button"
      onClick={() => { void promptInstall(); }}
      aria-label="Install Reign as an app"
      title="Install Reign"
      style={buttonStyle}
    >
      <Icon as={Download} size={16} />
      <span style={{ marginLeft: 6 }}>Install</span>
    </button>
```

Size 16 because the button is compact (0.875rem text). The brand default (20) would dwarf the label.

- [ ] **Step 2: Run existing InstallButton tests to verify no regression**

```bash
cd frontend && npx vitest run src/components/common/InstallButton.test.tsx
```

Expected: All four tests PASS. The `toHaveTextContent(/install/i)` assertion still matches because the "Install" `<span>` is still rendered.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/common/InstallButton.tsx
git commit -m "feat(frontend): InstallButton uses Lucide Download icon (#180)"
```

---

## Task 6: Document the convention in `frontend/CLAUDE.md`

**Files:**
- Modify: `frontend/CLAUDE.md`

- [ ] **Step 1: Append the Icons subsection under UI Primitives**

In `frontend/CLAUDE.md`, find the "UI Primitives" section. After the "Tailwind is retired." paragraph (~line 76), append:

```markdown
- **Icons.** Lucide via `lucide-react`. All icon usage goes through `shared/components/Icon` to inherit brand defaults (1.5 stroke, 20px size). Sites import the specific icon directly (`import { ArrowLeft } from 'lucide-react'`) and pass it via the `as` prop: `<Icon as={ArrowLeft} />`. Per-site overrides for `size` / `strokeWidth` are fine when needed (e.g. compact buttons). No emoji or Unicode glyphs for chrome icons — `BRAND_GUIDELINES.md` lines 437 + 692.
```

- [ ] **Step 2: Verify the section reads correctly**

```bash
sed -n '/^## UI Primitives/,/^## /p' frontend/CLAUDE.md | head -30
```

Expected: The Icons bullet appears as the last bullet under "UI Primitives", before the next `## ` heading.

- [ ] **Step 3: Commit**

```bash
git add frontend/CLAUDE.md
git commit -m "docs(frontend): document Lucide icon convention (#180)"
```

---

## Task 7: Run full unit-test suite + build verification

**No file changes; verification only.**

- [ ] **Step 1: Run the full frontend unit test suite**

```bash
cd frontend && npm test
```

Expected: All tests pass. Specifically watch for:
- `Icon.test.tsx` (4 new tests passing)
- `PageShell.test.tsx` (existing tests unchanged, still passing)
- `InstallButton.test.tsx` (existing tests unchanged, still passing)

If any test fails because it asserted on a literal glyph character (`←`, `☀`, `☾`), refactor the assertion to use `data-testid` or `aria-label`. Pre-flight grep suggested this won't happen — but the test suite is the ground truth.

- [ ] **Step 2: Run the frontend build**

```bash
cd frontend && npm run build
```

Expected: Build succeeds. No TypeScript errors. Bundle size delta should be in the low single-digit kilobytes (four icons + tiny wrapper).

If TypeScript complains about `LucideIcon` or `LucideProps`, check the installed version's `node_modules/lucide-react/dist/lucide-react.d.ts` for the canonical type names and adjust the wrapper import. (Lucide v1 has been stable on these names but always defer to the registry over memory.)

- [ ] **Step 3: Sanity-check the rendered SVGs in the dev server**

```bash
task dev:up
```

Open http://localhost:5180/, then:
- Verify the dark-mode toggle in the header shows a sun (light mode) or moon (dark mode) SVG.
- Click into any puzzle page where the back button appears — confirm it shows a left-arrow SVG.
- If the browser supports `beforeinstallprompt` (Chrome desktop / Edge), confirm the Install button shows a download icon alongside the "Install" text. (iOS Safari and other browsers don't fire the prompt — the button stays hidden as before.)

Tear down when satisfied:

```bash
task dev:down
```

This is a one-off visual smoke check, not a durable test. Per the `playwright-cli for one-off cross-boundary verifications` lesson, a durable e2e spec isn't warranted — this change doesn't cross a service boundary.

- [ ] **Step 4: No commit (verification task)**

---

## Task 8: Cross-link to #176

- [ ] **Step 1: Add a checkbox under #176's tooling/styling section**

```bash
gh issue view 176 --json body --jq .body > /tmp/176-body.md
```

Open `/tmp/176-body.md` and find the "### Tooling / styling" section. Add a new checkbox at the top of that section:

```markdown
- [ ] **Icon library adopted** — see #180. Lucide via `lucide-react`, wrapped by `shared/components/Icon`.
```

Then update the issue:

```bash
gh issue edit 176 --body-file /tmp/176-body.md
rm /tmp/176-body.md
```

- [ ] **Step 2: Verify the edit landed**

```bash
gh issue view 176 --json body --jq .body | grep -A 1 "Icon library adopted"
```

Expected: The new checkbox line is printed.

- [ ] **Step 3: No commit (GitHub-side change)**

---

## Final wrap-up (handled by `finishing-a-development-branch`)

After all tasks pass:

1. Run Superpowers `requesting-code-review` over the diff. Fix CRITICAL / HIGH findings (and apply the SWEEP grep to any cited site).
2. Decide on `security-review-final`: this diff doesn't touch auth, handlers, infra, or dependency files beyond a single ISC-licensed npm package — deep review unlikely needed but still confirm with the trigger list in `/CLAUDE.md`.
3. Open the PR with the "Key Decisions" section calling out:
   - Lucide already mandated by `BRAND_GUIDELINES.md`; this PR ratifies, doesn't re-litigate.
   - All three existing glyph sites migrate in one PR for visual consistency.
   - `<Icon>` wrapper (not direct lucide imports at every site) so brand defaults are enforced at a single seam.
4. Close #180 on merge.

---

## Self-review notes

- **Spec coverage:** All four AC items from #180 have a corresponding task: library chosen (Task 1), first icon adopted (Tasks 3/4/5), CLAUDE.md updated (Task 6), checkbox under #176 (Task 8).
- **Type consistency:** `IconProps` defined once in Task 2 with `as: LucideIcon`; all later tasks use that prop shape.
- **No placeholders:** every code step shows the exact code; every command shows the exact command and expected outcome.
- **Lesson 8 (registry-verified deps):** Task 1 Step 2 explicitly probes the installed package for the four named exports we depend on.
- **Lesson 12 (cross-boundary):** this diff stays within the frontend layer (no API calls, no service boundary crossed) — no Playwright e2e spec required.
