import { test, expect } from "@playwright/test";

/**
 * Asserts the dynamic mode-button list matches CONFIG rows with
 * enabled=true. The e2e fixture pool is seeded (by .localstack/
 * init-aws.sh) with:
 *
 *   7#standard enabled
 *   9#standard enabled
 *   9#double   DISABLED  ← the interesting case
 *
 * The disabled combo lets us assert the filter works end-to-end:
 * it is visible in admin pool listings but absent from the public
 * /api/config/modes response and therefore absent from the UI.
 *
 * Post-R-7-02 the preset buttons live on /curation (admin-only) via
 * <ProtectedAdminRoute>. Driving this test requires a Clerk admin
 * session injected from the test harness — the same scaffolding the
 * three skipped admin tests in auth.spec.ts are waiting on. Once
 * `@clerk/testing`'s session-injection helper is wired up there,
 * delete the skip below and route this test through /curation.
 */

const needsClerkAdminSession = true;

test.describe("e2e: dynamic mode buttons", () => {
  test("renders only enabled combos; disabled 9×9 Double is absent", async () => {
    test.skip(
      needsClerkAdminSession,
      "preset buttons moved to admin-gated /curation; needs Clerk admin session injection (see auth.spec.ts)",
    );
  });
});
