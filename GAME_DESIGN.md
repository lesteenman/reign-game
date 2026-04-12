# Reign — Vision Document

**Reign** (working title) — formerly "Queens Game"

This is the living design document for Reign. It describes the full product vision we're building toward. Individual features are implemented incrementally via the roadmap, but this document captures the north star.

---

## Concept

A puzzle game where players place markers on a colored grid such that exactly one marker occupies each row, column, and colored region — with no two markers adjacent (including diagonally). The game offers curated puzzles across difficulty levels, a daily challenge with speed-based leaderboards, and a premium tier for dedicated players.

The default presentation is minimalist and abstract — the identity comes from clean design and vibrant region colors, not from a specific real-world metaphor. Alternative themes (including a classic "Queens" chess theme) are available as cosmetic options.

Available as a Progressive Web App: installable on mobile, fully functional on desktop, playable offline for practice mode.

---

## Game Modes

### Standard Mode

Place **one queen** per row, column, and colored region. No two queens may be adjacent (horizontally, vertically, or diagonally).

| Difficulty | Grid Size | Region Count | Character |
|------------|-----------|--------------|-----------|
| Easy       | 5x5       | 5            | Simple region shapes, few ambiguous placements, solvable with basic elimination |
| Medium     | 7x7       | 7            | Irregular region shapes, requires systematic elimination logic |
| Hard       | 9x9       | 9            | Complex interlocking regions, requires multi-step deduction chains |

### Double Queens Mode

Place **two queens** per row, column, and colored region. The adjacency constraint still applies — no two queens may touch, even diagonally. This dramatically increases difficulty since even small grids become challenging.

Uses the same grid sizes (5x5, 7x7, 9x9) but the constraint density is much higher. A 5x5 Double Queens puzzle is comparable in difficulty to a 7x7 Standard puzzle.

### Practice Play

- Unlimited curated puzzles per difficulty level
- Puzzles drawn from the curated puzzle database
- No time pressure, no leaderboard
- Available offline (puzzles cached locally)
- Progress tracked locally (puzzles solved, current streak)

### Daily Challenge

- One puzzle per difficulty level per day, per mode = **6 daily puzzles** (3 Standard + 3 Double Queens)
- Same puzzle for all players worldwide (seeded by date)
- Timer starts on first cell interaction
- Scoring: speed-based percentile rank + absolute position (e.g., "512th out of 1,247 players")
- Results visible after completion (no peeking at leaderboard mid-solve)
- Daily streak tracking

---

## Puzzle Design

### Generation

Puzzles are **algorithmically generated, then human-curated**. The generator is a core piece of the project and will be open source.

**Generator requirements:**
- Produce puzzles that have exactly one valid solution (Standard) or a controlled number of solutions (Double Queens)
- Control difficulty through: grid size, region shape complexity, number of forced vs. deducible placements, depth of required deduction chains
- Output puzzle definition (grid size, region map) and solution
- Reject puzzles that are trivially solvable by a single strategy (e.g., only naked singles)
- Support both Standard and Double Queens constraint sets

**Curation pipeline:**
1. Generator produces candidate puzzles in batch
2. Solver verifies uniqueness and rates difficulty
3. Human curator reviews, tags, and approves puzzles for the database
4. Approved puzzles are assigned to practice pools or scheduled as daily puzzles

**Difficulty rating factors:**
- Grid size (primary factor)
- Region shape irregularity (more irregular = harder)
- Deduction chain depth (how many steps of elimination before a placement is forced)
- Number of "decision points" where multiple strategies could apply
- Backtracking requirement (puzzles that require trial-and-error are harder)

### Puzzle Database

- Curated puzzles are the content moat — proprietary, not open source
- Stored server-side, served via API
- Practice puzzles cached locally for offline play
- Daily puzzles fetched fresh each day
- Puzzle IDs are stable (for sharing, leaderboard references)

---

## Scoring & Leaderboards

### Daily Challenge Scoring

- **Metric:** Completion time (seconds from first interaction to correct solution)
- **Percentile rank:** Where you fall relative to all players who completed the same daily puzzle (e.g., "Top 15%")
- **Absolute position:** Your rank number out of total completions (e.g., "512 / 1,247")
- Leaderboard updates in near-real-time
- Results only visible after you complete the puzzle (anti-spoiler)

### Personal Stats (tracked locally + synced for authenticated users)

- Total puzzles solved (by mode and difficulty)
- Average solve time (by mode and difficulty)
- Current streak (consecutive days with at least one daily puzzle completed)
- Best streak
- Daily challenge history

---

## User Identity

### Phase 1 (MVP)

- Anonymous device-linked identity
- Local stats only
- Daily leaderboard participation via anonymous device ID

### Phase 2

- Optional account creation (OAuth — Google, Apple)
- Stats sync across devices
- Persistent leaderboard identity (display name)
- Account required for premium features

---

## Monetization

### Model: Freemium (no ads, ever)

Pay for additional curated content, not for removing annoyances.

**Free tier:**
- Daily challenge (all 6 puzzles) with leaderboard
- Limited practice puzzle pool (rotating selection)
- Basic personal stats
- Full offline support for available puzzles

**Premium tier (subscription — pricing TBD):**
- Unlimited access to the full curated puzzle archive
- Detailed stats and history (solve time trends, difficulty progression)
- Extended streak tracking and achievements
- Premium visual themes (Gems, Garden, Neon, Cosmos, and more)
- Priority access to new puzzle types/modes as they're added

**What we never do:**
- No ads
- No pay-to-win (daily challenge is always free and identical for all players)
- No consumable in-app purchases
- No data selling

---

## Technical Architecture

### Frontend (PWA)

| Concern | Technology |
|---------|-----------|
| Framework | React 18 + TypeScript |
| Bundler | Vite |
| Styling | Tailwind CSS |
| PWA | Workbox (service worker, offline caching, install prompt) |
| State | Local state + React Context (no heavy state library needed) |
| Testing | Vitest (unit) + Playwright (e2e) |
| Hosting | S3 + CloudFront |

**Key frontend principles:**
- Mobile-first responsive design
- Touch-optimized grid interaction (tap to place/remove queen)
- Smooth animations for placement, error highlighting, completion
- Offline-capable: service worker caches app shell + practice puzzles
- Installable as PWA on iOS, Android, desktop

### Backend (Serverless)

| Concern | Technology |
|---------|-----------|
| Language | Go |
| Runtime | AWS Lambda |
| API | API Gateway (REST) |
| Auth | Cognito (Phase 2) |
| Database | DynamoDB (on-demand pricing) |
| IaC | Terraform |
| CI/CD | GitHub Actions |

**Key backend responsibilities:**
- Serve puzzle data (daily + practice)
- Record daily challenge completions and compute leaderboard
- Manage user accounts and premium subscriptions (Phase 2)
- Puzzle generation pipeline (batch Lambda jobs)
- Admin API for puzzle curation

### Data Model (high-level)

**Puzzle:**
- PuzzleID (partition key)
- GridSize, Mode (Standard/Double), Difficulty
- RegionMap (2D array of region IDs)
- Solution (encrypted, never sent to client before completion)
- Status (draft/approved/daily-scheduled/archived)
- CreatedAt, CuratedBy

**DailyPuzzle:**
- Date + Mode + Difficulty (composite key)
- PuzzleID (reference)

**Completion:**
- PuzzleID + UserID (composite key)
- CompletionTime (seconds)
- CompletedAt
- IsDaily (boolean)

**User (Phase 2):**
- UserID
- DisplayName, AuthProvider
- SubscriptionTier, SubscriptionExpiry
- Stats (embedded: total solved, streaks, averages)

### Infrastructure

- **Single environment initially**, split to dev/prod later
- S3 for static frontend assets
- CloudFront CDN in front of S3
- API Gateway + Lambda for backend
- DynamoDB tables with on-demand billing
- All infrastructure managed via Terraform
- GitHub Actions: CI on PR, CD on merge to main
- Cost target: minimize at low traffic, scales with usage

---

## Open Source Strategy

### Open Core Model

| Component | License | Rationale |
|-----------|---------|-----------|
| Puzzle generator + solver | MIT | Core algorithm — builds community, attracts contributors, discourages re-implementation by making it free |
| Frontend app shell + UI components | MIT | Lowers barrier to contribution |
| Curated puzzle database | Proprietary | Content moat — this is the product's value |
| Daily puzzle service + leaderboard API | Proprietary (hosted) | SaaS layer — the service, not the code |
| Premium features | Proprietary | Monetization surface |

The puzzle generator being open source is a deliberate strategic choice: a high-quality generator is hard to build, and open-sourcing it earns credibility while the curated database and hosted service remain the competitive advantage. Contributors can self-host the engine, but the *service* is what people pay for.

---

## Theme System

Themes are coherent visual packages that change the entire look and feel of the game. They are purely cosmetic — gameplay mechanics are identical across all themes.

### What a Theme Controls

| Layer | Description |
|-------|-------------|
| Piece icon | The marker placed on the grid (dot, queen, gem, flower, etc.) |
| Grid lines | Line weight, color, style (thin/muted, traditional, neon, hand-drawn) |
| Region rendering | How colored regions are drawn (solid fill, gradients, patterns, textures) |
| Background | Screen background behind the grid (clean white/dark, parchment, nature, space) |
| Placement animation | Visual feedback when placing a marker (fade-in, drop, sparkle, bloom) |
| Completion animation | Celebration when the puzzle is solved (ripple, crown flourish, fireworks) |
| Conflict highlight | How constraint violations are shown (red pulse, glow, shake) |
| Color palette | Region colors — each theme ships its own colorblind-safe palette |

### Default Theme: Minimalist

The default theme is abstract and clean. It should feel premium on its own — confident, modern, no ornamentation needed.

- **Piece:** Filled circle / pip
- **Grid:** Thin, muted lines
- **Regions:** Solid pastel fills with subtle borders
- **Background:** Clean white (light mode) or dark neutral (dark mode)
- **Animations:** Subtle fade-in on placement, clean ripple on completion
- **Identity:** The brand comes from the grid and region colors, not from a specific metaphor

### Built-in Theme: Queens (Classic)

A chess-inspired theme available to all players for free.

- **Piece:** Chess queen icon
- **Grid:** Traditional board lines
- **Regions:** Solid bold fills
- **Background:** Parchment texture
- **Animations:** Drop with bounce on placement, crown flourish on completion

### Premium Themes (examples, not exhaustive)

Premium themes are part of the paid tier. Each is a fully designed visual package:

- **Gems** — Faceted gem icons, crystalline grid, prismatic completion effect
- **Garden** — Flower icons, organic grid lines, bloom animation, nature background
- **Neon** — Glowing dot icons, neon grid, electric pulse animations, dark background
- **Cosmos** — Star icons, constellation grid, nebula background, starburst completion

### Theme Architecture Notes

- Theme system must be designed into the component architecture from the start (not bolted on)
- Each theme is a data object: piece SVG/component, color palette, animation config, background asset
- Components consume theme tokens from React Context
- Theme selection persisted locally; synced to account when authenticated
- Placeholder art acceptable initially — Nano Banana 2 prompts written alongside for final assets

---

## UX Principles

1. **Instant engagement** — Grid loads immediately, no onboarding wall
2. **One-hand playable** — All interactions reachable with thumb on mobile
3. **Clear feedback** — Invalid placements highlighted immediately, completion celebrated
4. **No friction to daily play** — Daily puzzle is one tap from home screen
5. **Respect the player's time** — No unskippable animations, no artificial delays
6. **Accessible** — WCAG 2.1 AA minimum, colorblind-friendly region palettes, keyboard navigation on desktop
7. **Visually distinctive** — The minimalist default should be recognizable at a glance; themes add personality without clutter

---

## Future Considerations (not in initial scope)

These are ideas worth tracking but explicitly out of scope for the initial build:

- **Tournament mode** — Timed competitive events
- **Puzzle creator** — Let users design and share puzzles
- **Additional constraint variants** — Knight's move restriction, diagonal regions
- **Larger grids** — 11x11, 13x13 for extreme difficulty
- **Social features** — Friends, challenges, shared stats
- **Native apps** — If PWA limitations become blocking
- **Seasonal/limited themes** — Time-limited premium themes tied to events
- **Theme creator** — Let premium users customize their own color palette

---

*This document evolves as we iterate. Changes should be discussed before being applied.*
