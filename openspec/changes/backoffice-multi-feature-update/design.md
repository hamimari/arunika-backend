## Context

The `arunika_backoffice` is a React 19 + TypeScript admin panel backed by Ant Design, React Query, and Axios. It manages content (AR cards, dongeng/fairy tales, categories, tracing, counting, badges, banners), premium packages, users, payments, and campaigns.

Five issues have been identified:
1. Analytics endpoints (`/admin/analytics/dau`, `/admin/analytics/new-users`) return 500 errors on the frontend — likely a missing analytics page or broken API wiring since no analytics route exists in the app.
2. The AR cards form does not expose `image_url` or `printable_img` fields despite the backend supporting them.
3. There is no UI for managing AR card categories — the `categoriesApi` exists but is only used in content pages as a lookup; there is no dedicated category CRUD menu.
4. The premium packages page filters out inactive packages, making them invisible to admins.
5. The dongeng (fairy tales) page has no mechanism for managing individual pages within a story.

All five are isolated, UI-layer changes with no new dependencies or backend modifications required.

## Goals / Non-Goals

**Goals:**
- Fix the 500 error on analytics by wiring the analytics page/route correctly
- Add `image_url` and `printable_img` fields to the AR card create/edit modal
- Add a dedicated AR Card Categories page with add, update, hide/show, and delete actions
- Show all premium packages (including inactive) in the admin list
- Add page management (add, update, delete) within the Dongeng detail view

**Non-Goals:**
- No backend changes — all fixes are frontend-only
- No new API client functions beyond what's necessary to support the UI
- No redesign of existing pages beyond the specified additions
- No change to authentication or permissions model

## Decisions

### 1. Analytics: Add a dedicated `/analytics` route
**Decision:** Create `src/pages/analytics/AnalyticsPage.tsx` and register it at `/analytics` in `App.tsx`.
**Rationale:** The analytics API already exists. The 500 errors are most likely caused by the frontend hitting the endpoints incorrectly (bad auth headers, wrong params) or no page exists at all. A dedicated page with proper React Query wiring and error handling will surface the real error and fix the integration.
**Alternative considered:** Debug directly in `DashboardPage.tsx` — rejected because analytics warrants its own route for navigability.

### 2. AR Card image fields: Extend existing modal
**Decision:** Add `image_url` (text input) and `printable_img` (text input) to the existing create/edit `Form` in `ArCardsPage.tsx`.
**Rationale:** Minimal change — the modal already handles all other fields; adding two `Form.Item` entries is sufficient. No new component needed.

### 3. AR Card Categories: New page reusing content page pattern
**Decision:** Create `src/pages/content/ArCardCategoriesPage.tsx` mirroring the structure of `CategoriesPage.tsx`, using `categoriesApi` (which already supports list/create/update/delete/toggleVisibility).
**Rationale:** The API is already built. The existing `CategoriesPage` may already cover general categories — a dedicated AR card categories page provides focused management. If `categoriesApi` already covers AR card categories, no new API calls are needed.
**Alternative considered:** Extend `CategoriesPage.tsx` with a tab — rejected to keep routing clean and each page focused.

### 4. Premium packages: Remove active filter
**Decision:** Remove any `active: true` / status filter from the React Query list call in `PremiumPackagesPage.tsx`. Show inactive packages with a visual indicator (e.g., greyed row or "Inactive" badge).
**Rationale:** Admins need to see all packages to manage them. The existing `toggleVisibility` action already handles activate/deactivate.

### 5. Dongeng pages: Inline nested page management
**Decision:** Add a collapsible "Pages" section within the dongeng detail/edit drawer or modal in `FairyTalesPage.tsx`. Pages can be added, edited, or deleted inline. Use `fairyTalesApi` extended with page sub-resource calls if needed, or a new `fairyTalePagesApi`.
**Rationale:** Keeping page management inline with the parent dongeng keeps the UX simple without requiring a new route. If the API supports `/admin/content/fairy-tales/:id/pages`, a small API helper suffices.

## Risks / Trade-offs

- **Analytics 500 errors may be backend bugs** → The frontend fix will expose cleaner error messages; if the API itself is broken, the error state will be visible and actionable.
- **`categoriesApi` may serve all content types, not just AR cards** → Review the API response to confirm filtering by type is needed; if categories are global, the new page lists all categories, which is acceptable.
- **Dongeng pages API may not exist** → If `/admin/content/fairy-tales/:id/pages` is not implemented, page management cannot be added without a backend change. Risk is low — investigate during implementation.
- **No rollback needed** — all changes are additive UI features with no data migrations or breaking changes.
