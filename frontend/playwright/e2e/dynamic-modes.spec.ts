import { test, expect } from "@playwright/test";
import {
  containsCombo,
  type ModesResponse,
} from "../test-helpers/modes";

/**
 * Asserts the public dynamic mode list reflects CONFIG enabled flags.
 *
 * Sentinels (post slice e2e-coverage-and-clerk-injection devops chunk 1):
 *   7#standard enabled
 *   9#standard enabled
 *   9#double   enabled  ← previously the disabled sentinel; flipped on
 *   7#double   DISABLED ← the new disabled sentinel (KI-007 still infeasible)
 *
 * `/api/config/modes` is the public, unauthenticated surface that drives
 * landing-page mode buttons for non-admin players. The disabled combo
 * lets us assert the filter works end-to-end: it MUST be present in
 * admin pool listings (not asserted here) but absent from this public
 * response. Per spec EC-04 and design D6.
 *
 * No Clerk session needed — this hits a public endpoint.
 */

test.describe("e2e: dynamic mode buttons", () => {
  test("public modes list includes 9#double (enabled) and excludes 7#double (disabled sentinel)", async ({
    request,
  }) => {
    const response = await request.get("/api/config/modes");
    expect(response.ok()).toBe(true);

    const body = (await response.json()) as ModesResponse;
    expect(Array.isArray(body.modes)).toBe(true);

    // 9#double is enabled post-devops chunk 1 — must surface to players.
    expect(containsCombo(body.modes, 9, "double")).toBe(true);

    // 7#double is the new disabled sentinel — must be filtered out by
    // the public endpoint even though the row exists in CONFIG.
    expect(containsCombo(body.modes, 7, "double")).toBe(false);

    // Sanity: the always-on combos remain visible.
    expect(containsCombo(body.modes, 7, "standard")).toBe(true);
    expect(containsCombo(body.modes, 9, "standard")).toBe(true);
  });
});
