# Spec: Frontend Scaffold

Covers R-001 (frontend directory) and R-003 (React + Vite scaffold).

## Requirements

### FS-01: Package Initialization

- `frontend/package.json` exists
- Dependencies: `react` 19.x, `react-dom` 19.x
- Dev dependencies: `typescript` 6.x, `vite` 8.x, `tailwindcss` 4.2.2, `vitest`, `@types/react`, `@types/react-dom`, `@vitejs/plugin-react`

### FS-02: Vite Configuration

- `frontend/vite.config.ts` exists with React plugin configured
- Build output directory: `dist/`

### FS-03: TypeScript Configuration

- `frontend/tsconfig.json` exists with `strict: true`
- No `any` type in scaffold code
- Appropriate React JSX configuration for React 19

### FS-04: Tailwind CSS

- CSS-first configuration (Tailwind 4 style — no `tailwind.config.ts`)
- Global CSS file uses `@import "tailwindcss"` or equivalent Tailwind 4 import syntax
- Vite processes Tailwind correctly during `npm run build`

### FS-05: App Placeholder

- `frontend/src/App.tsx` exists, renders a heading with "Reign"
- `frontend/src/main.tsx` mounts App to a root DOM element
- `frontend/index.html` exists with root div and references `main.tsx`

### FS-06: Vitest

- Vitest configured (in `vite.config.ts` or separate `vitest.config.ts`)
- At least one passing test: `App.test.tsx` verifies the component renders without crashing
- `npm test` script defined in `package.json` and runs Vitest

### FS-07: PWA Manifest Stub

- `frontend/public/manifest.json` exists
- Contains: `name`, `short_name`, `start_url`, `display: "standalone"`
- Placeholder icon referenced (simple SVG or PNG)

### FS-08: Build Verification

- `npm run build` succeeds, produces `dist/` containing `index.html`
- `npm test` passes all tests
- `npm run dev` starts Vite dev server without errors
- No TypeScript compilation errors
- No console warnings in dev mode

## Acceptance Criteria

All FS-01 through FS-08 requirements pass. The frontend builds, tests pass, and the dev server starts.
