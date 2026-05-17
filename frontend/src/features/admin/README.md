# `src/features/admin/`

The admin feature contains the puzzle-pool management UI and its route guard. Entry point is `pages/AdminPage`, mounted at `/admin` by `App.tsx` and wrapped by `components/ProtectedAdminRoute`. `AdminLandingPage` handles the two unauthorised states (anonymous and forbidden). `ProtectedAdminRoute` reads Clerk auth state and renders the appropriate page; it imports `AdminPage` and `AdminLandingPage` as intra-feature siblings from `../pages/`, keeping the cross-layer dependency that previously existed between `components/auth/` and `pages/` entirely within this feature.
