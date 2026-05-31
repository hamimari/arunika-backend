## Context

Arunika is a children's education mobile app built in Flutter. Its Go backend exposes REST APIs consumed by the mobile app. The backend covers: user auth, AR cards, categories, fairy tales (dongeng), tracing, counting, badges, payments (Midtrans), push notifications (FCM), and child growth tracking.

Currently there is no admin interface. Content updates require direct database changes. There is no visibility into user activity or payment health. The backoffice project directory (`arunika_backoffice`) is currently empty, ready for the new React app.

## Goals / Non-Goals

**Goals:**
- React SPA admin panel with role-protected routes
- Admin authentication separate from mobile user flow
- Full CRUD + visibility toggle for all content types (fairy tales, AR cards, tracing items, counting questions, badges, categories)
- User list and individual user detail with payment history
- Dashboard with DAU, new user count, popular features, payment success/failure rates
- Campaign tool: send push or email notifications to all or filtered users
- Payment receipt notifications triggered automatically via Midtrans webhook
- New `/admin` API group in the existing Go backend

**Non-Goals:**
- Mobile app code changes
- Multi-tenant admin (single admin role is sufficient for now)
- Real-time WebSocket dashboard (polling every 30s is acceptable)
- A/B testing or feature flag infrastructure
- Media CDN — existing asset URLs remain managed externally

## Decisions

### D1: Extend existing Go backend with `/admin` routes vs. separate admin service

**Decision:** Extend the existing Go backend.

**Rationale:** The existing backend already has all the database models, services, and the notification/payment infrastructure. Adding an `/admin` route group with a separate admin middleware avoids duplicating business logic. A second service would require duplicated DB connections, shared models, and deployment complexity for a small team.

**Alternative considered:** Separate Go admin microservice — rejected due to operational overhead.

### D2: Admin auth strategy

**Decision:** New `admin_users` table with email + bcrypt password. Admin JWT uses a separate signing claim (`role: admin`) validated by a new `AdminAuthMiddleware`. Mobile user tokens cannot access admin routes.

**Rationale:** Decoupling admin identity from mobile user identity prevents privilege escalation if a user token is compromised.

### D3: Analytics storage

**Decision:** Compute analytics from existing tables using SQL aggregations (no separate metrics store for now). A `user_sessions` table will be added to track daily logins for DAU. Payment metrics come from the existing `transactions` table. Feature popularity is derived from activity tables (tracing_progress, counting_progress, ar views).

**Rationale:** Keeps infrastructure simple. If query performance degrades, a materialized view or read replica can be added later.

### D4: Frontend stack

**Decision:** React + TypeScript, Vite for bundling, React Router for navigation, React Query for data fetching + caching, Ant Design (antd) for UI components (tables, forms, charts), Chart.js via react-chartjs-2 for dashboard graphs.

**Rationale:** Ant Design provides production-ready admin components (DataTable, Form, Modal) that reduce custom UI work. React Query handles polling for dashboard refresh cleanly.

### D5: Campaign notifications

**Decision:** Reuse the existing `NotificationService` and FCM token registry. New admin endpoint `POST /admin/campaigns` accepts a message payload and optional user filter, then batch-dispatches FCM push and/or email (via existing template engine in `/templates`).

**Rationale:** The notification infrastructure already exists; no new dependency needed.

### D6: Content visibility toggle

**Decision:** Add a `hidden boolean DEFAULT false` column to each content table (fairy_tales, ar_cards, tracing_items, counting_questions). The mobile app's read endpoints filter `WHERE hidden = false`. Admin toggle endpoints flip this flag.

**Rationale:** Soft-hiding is safer than deletion and allows quick re-enabling without data recovery.

## Risks / Trade-offs

- **SQL aggregation performance for DAU** → Mitigation: add index on `user_sessions(created_at)` and cache the result for 60s in Redis (already present in infrastructure).
- **Admin credentials in new table** → Mitigation: bcrypt cost factor 12, no password recovery via mobile flow, HTTPS enforced.
- **Schema migration on live DB** → Mitigation: Flyway (already used) for all migrations; `hidden` column defaults to `false` so existing data is unaffected.
- **Batch FCM campaign rate limits** → Mitigation: dispatch in chunks of 500 with exponential backoff; log failures per user token.
- **Single admin role** → Trade-off: no granular permissions. Acceptable for current team size; RBAC can be added later.

## Migration Plan

1. Run Flyway migrations: `admin_users`, `user_sessions`, `hidden` columns on content tables.
2. Deploy updated Go backend with new `/admin` routes (backwards compatible — mobile routes unchanged).
3. Deploy React backoffice app (static hosting or same server under `/admin` path).
4. Seed first admin user via a one-time CLI script or manual DB insert with bcrypt hash.
5. Rollback: Flyway down scripts revert schema; feature flag on `AdminAuthMiddleware` can disable admin routes without redeployment.

## Open Questions

- Should DAU count unique logins per calendar day (UTC) or sliding 24h window? → Recommend calendar day for simplicity.
- Email sending: is SMTP configured in the backend, or is only FCM push available today? → Check `/templates` and env config before implementing email campaign path.
- Should deleted content be hard-deleted or soft-deleted? → Recommend soft-delete (add `deleted_at` column) for auditability.
