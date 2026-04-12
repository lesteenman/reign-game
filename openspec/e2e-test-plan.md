# E2E Test Plan

Comprehensive end-to-end test plan for Playwright testing against the live application. This is a living document — new test cases are added with every feature that goes through e2e testing.

---

## Test Users

<!-- TODO: Define test users with different configurations to exercise edge cases.

| User | Email | Role | Notes |
|------|-------|------|-------|
| Admin | admin@example.com | ADMIN | Full access, seeds data |
| Manager | manager@example.com | MANAGER | Approvals, team overview |
| Employee A | employee.a@example.com | USER | Standard user, baseline |
| Employee B | employee.b@example.com | USER | Edge case (e.g., part-time, new hire) |
-->

## Test Phases

### Phase 0: Setup (sequential)
<!-- Seed data, create test users, configure settings -->

### Phase 1: Independent User Tests (parallel)
<!-- Each user tests their own flows. Safe to parallelize because users operate on isolated data. -->

### Phase 2: Cross-User Tests (sequential)
<!-- Approvals, admin operations, audit trail verification -->

---

## Test Angles

Every feature should be tested from multiple angles. When adding test cases, consider:

1. **Happy path** — the primary user flow as designed, step-by-step
2. **Error and edge cases** — invalid data, empty forms, boundary values, double-clicks, navigating away mid-flow
3. **Role-based access** — how the flow differs per role, what unauthorized users see, permission changes mid-session
4. **Multi-step and state** — flows spanning multiple pages, preconditions, back navigation, refresh, new tab
5. **Cross-user interactions** — one user creates/another approves, concurrent edits, data visibility across roles

---

## Test Groups

<!-- TODO: Add test scenarios grouped by feature. Example:

### Group 1: Authentication
| # | Scenario | Angle | User | Steps | Expected |
|---|----------|-------|------|-------|----------|
| 1.1 | Login with valid credentials | Happy path | Employee A | Navigate to /login, enter email/password, click Submit | Dashboard loads, user name shown in header |
| 1.2 | Login with wrong password | Error | Employee A | Navigate to /login, enter wrong password, click Submit | Error message "Invalid credentials" shown |
| 1.3 | Access protected page without login | Access | — | Navigate directly to /dashboard | Redirect to /login |
| 1.4 | Session persistence after refresh | State | Employee A | Login, refresh the page | Still logged in, dashboard still shown |

### Group 2: Order Management
| # | Scenario | Angle | User | Steps | Expected |
|---|----------|-------|------|-------|----------|
| 2.1 | Create a new order | Happy path | Employee A | Click "New Order", fill all fields, click Submit | Order appears in list with correct details |
| 2.2 | Submit order with missing required field | Error | Employee A | Click "New Order", leave required field empty, click Submit | Validation error shown, form not submitted |
| 2.3 | Manager views employee's order | Cross-user | Manager | Login as manager, navigate to team orders | Employee A's order visible with correct status |
-->

---

## Bug-Found Protocol

When an e2e test finds a bug, follow this process before fixing:

1. **Document the finding** — spec, scenario, expected vs. actual, severity, screenshot
2. **Write a unit test first** — capture the failure at the unit level. This test should FAIL with current code.
3. **Fix the bug** — minimal fix targeting only the broken code path
4. **Verify the unit test passes** — run the unit test suite
5. **Re-run the e2e scenario** — verify it passes through the real UI
6. **Continue testing** — only after both unit and e2e tests pass
