import { test, expect, type APIRequestContext } from "@playwright/test";
import { clerk } from "@clerk/testing/playwright";

/**
 * Admin CONFIG-mutation flow e2e (slice e2e-coverage-and-clerk-injection,
 * EC-08).
 *
 * Flow: admin signs in -> PUT /api/admin/config/{size}/{mode} flips the
 * enabled flag -> GET /api/config/modes reflects the change -> cleanup
 * flips it back. Toggle target is `7#double`, the disabled sentinel
 * seeded by .localstack/init-aws.sh (enabled=false).
 *
 * Cleanup uses try/finally inside the test body, not test.afterAll: the
 * afterAll hook has no signed-in admin session because globalSetup
 * seeds the Clerk testing token but not a storage state. Mutation,
 * assertion, and restore all need the same admin JWT, so they share
 * one test body and one clerk.signIn call.
 *
 * 7#double is the only safe target. Every other seeded combo
 * (7#standard, 9#standard, 9#double) starts enabled=true; flipping
 * those to false would race with dynamic-modes.spec.ts on cleanup
 * failure. The e2e project runs fullyParallel:false, workers:1
 * (playwright.config.ts), so no other spec runs concurrently in
 * practice — but the choice of toggle target makes the spec resilient
 * to that invariant ever loosening.
 *
 * EC-08 lists this spec as parallel; we additionally wrap in
 * test.describe.serial so future additions in this file inherit
 * single-in-flight semantics.
 *
 * Env vars: E2E_CLERK_TEST_ADMIN_EMAIL, E2E_CLERK_TEST_ADMIN_PASSWORD.
 */

const TOGGLE_SIZE = 7;
const TOGGLE_MODE = "double";

type ConfigBody = {
  threshold: number;
  enabled: boolean;
  maxAttempts?: number;
};

type ConfigView = ConfigBody & {
  size: number;
  mode: string;
};

type Mode = { size: number; mode: string };
type ModesResponse = { modes: Mode[] };

type ComboStatus = {
  size: number;
  mode: string;
  config: ConfigBody;
  readyCount: number;
};
type AdminPoolResponse = { combos: ComboStatus[] };

const containsCombo = (modes: Mode[], size: number, mode: string): boolean =>
  modes.some((m) => m.size === size && m.mode === mode);

/**
 * Fetch the current CONFIG body for `size#mode` via the admin pool
 * endpoint (no GET-by-combo route exists today; the pool listing
 * carries every combo's config).
 */
async function readConfig(
  request: APIRequestContext,
  size: number,
  mode: string,
): Promise<ConfigBody> {
  const res = await request.get("/api/admin/pool");
  expect(res.ok(), `GET /api/admin/pool failed: ${res.status()}`).toBe(true);
  const body = (await res.json()) as AdminPoolResponse;
  const combo = body.combos.find((c) => c.size === size && c.mode === mode);
  if (!combo) {
    throw new Error(
      `combo ${size}#${mode} missing from /api/admin/pool — seed/CONFIG ` +
        `fixture not loaded`,
    );
  }
  return combo.config;
}

/**
 * Flip the `enabled` flag on `size#mode` while preserving threshold and
 * maxAttempts. Returns the updated ConfigView from the response so the
 * caller can assert on it.
 */
async function setEnabled(
  request: APIRequestContext,
  size: number,
  mode: string,
  enabled: boolean,
  source: ConfigBody,
): Promise<ConfigView> {
  const res = await request.put(`/api/admin/config/${size}/${mode}`, {
    data: {
      threshold: source.threshold,
      enabled,
      maxAttempts: source.maxAttempts ?? 0,
    },
  });
  expect(
    res.ok(),
    `PUT /api/admin/config/${size}/${mode} failed: ${res.status()}`,
  ).toBe(true);
  return (await res.json()) as ConfigView;
}

test.describe.serial("Admin CONFIG-mutation flow", () => {
  test("admin toggles 7#double enabled flag and public modes list reflects the change", async ({
    page,
    request,
  }) => {
    // Pre-flight: seed contract intact before any sign-in. Public
    // endpoint, no admin session needed; fails fast on fixture drift.
    const baseline = (
      await (await request.get("/api/config/modes")).json()
    ) as ModesResponse;
    expect(
      containsCombo(baseline.modes, TOGGLE_SIZE, TOGGLE_MODE),
      "seed contract: 7#double should start absent from /api/config/modes",
    ).toBe(false);

    // Sign in as admin (mirrors pool-replenishment.spec.ts pattern).
    await page.goto("/");
    await clerk.signIn({
      page,
      signInParams: {
        strategy: "password",
        identifier: process.env.E2E_CLERK_TEST_ADMIN_EMAIL!,
        password: process.env.E2E_CLERK_TEST_ADMIN_PASSWORD!,
      },
    });

    // Snapshot config so cleanup restores exactly and the PUT body
    // preserves threshold + maxAttempts (required by validateConfigBody).
    const before = await readConfig(request, TOGGLE_SIZE, TOGGLE_MODE);
    expect(
      before.enabled,
      "seed contract: 7#double config row should start enabled=false",
    ).toBe(false);

    try {
      // Flip enabled true. The PUT response is the authoritative ack.
      const updated = await setEnabled(
        request,
        TOGGLE_SIZE,
        TOGGLE_MODE,
        true,
        before,
      );
      expect(updated.size).toBe(TOGGLE_SIZE);
      expect(updated.mode).toBe(TOGGLE_MODE);
      expect(updated.enabled).toBe(true);
      expect(updated.threshold).toBe(before.threshold);

      // /api/config/modes reads the same CONFIG rows so the change is
      // visible immediately — no eventual-consistency window.
      const afterFlip = (
        await (await request.get("/api/config/modes")).json()
      ) as ModesResponse;
      expect(
        containsCombo(afterFlip.modes, TOGGLE_SIZE, TOGGLE_MODE),
        "after flip: /api/config/modes must include 7#double",
      ).toBe(true);
    } finally {
      // Restore enabled=false so dynamic-modes (which asserts 7#double
      // is absent) is not disturbed by subsequent runs. Runs on
      // mid-test failure too so a partial flip never leaks.
      await setEnabled(request, TOGGLE_SIZE, TOGGLE_MODE, false, before);
    }
  });
});
