## 1. Database Migrations (Backend)

- [x] 1.1 Create Flyway migration: `admin_users` table (id, email, password_hash, created_at)
- [x] 1.2 Create Flyway migration: `user_sessions` table (id, user_id, created_at) for DAU tracking
- [x] 1.3 Create Flyway migration: add `hidden boolean DEFAULT false` column to `fairy_tales`, `ar_cards`, `tracing_items`, `counting_questions`, `badges`, `categories`
- [x] 1.4 Create Flyway migration: add `deleted_at timestamp` column to all content tables for soft-delete
- [x] 1.5 Add indexes on `user_sessions(created_at)` and `transactions(created_at, status)`
- [ ] 1.6 Run and verify all migrations against local database

## 2. Admin Auth (Backend)

- [x] 2.1 Create `AdminUser` model and repository in the backend
- [x] 2.2 Implement `AdminAuthService` with login (bcrypt verify) and JWT issuance (role: admin claim)
- [x] 2.3 Implement `AdminAuthMiddleware` that validates admin JWT and rejects mobile user tokens with 403
- [x] 2.4 Implement `POST /admin/auth/login`, `POST /admin/auth/refresh`, `POST /admin/auth/logout` endpoints
- [x] 2.5 Add admin user seeding script (CLI or one-time migration) for initial admin account
- [x] 2.6 Write unit tests for AdminAuthService and middleware

## 3. Admin Content API (Backend)

- [x] 3.1 Add admin content CRUD handler: fairy tales (`GET/POST /admin/content/fairy-tales`, `GET/PUT/DELETE/PATCH /admin/content/fairy-tales/:id`)
- [x] 3.2 Add admin content CRUD handler: AR cards (`/admin/content/ar-cards`)
- [x] 3.3 Add admin content CRUD handler: tracing items (`/admin/content/tracing-items`)
- [x] 3.4 Add admin content CRUD handler: counting questions (`/admin/content/counting-questions`)
- [x] 3.5 Add admin content CRUD handler: badges (`/admin/content/badges`)
- [x] 3.6 Add admin content CRUD handler: categories (`/admin/content/categories`)
- [x] 3.7 Implement `PATCH /admin/content/{type}/:id/visibility` endpoint for toggle hidden flag
- [x] 3.8 Implement soft-delete: `DELETE` sets `deleted_at`; all mobile read queries filter `WHERE deleted_at IS NULL`
- [x] 3.9 Update all existing mobile content read endpoints to filter `WHERE hidden = false AND deleted_at IS NULL`

## 4. Admin Analytics API (Backend)

- [x] 4.1 Instrument user login handler to insert a row into `user_sessions` on each successful login
- [x] 4.2 Implement `GET /admin/analytics/dau?days=N` endpoint (unique user_id per calendar day from user_sessions)
- [x] 4.3 Implement `GET /admin/analytics/new-users?days=N` endpoint (count from users table grouped by created_at date)
- [x] 4.4 Implement `GET /admin/analytics/popular-features` endpoint (aggregate activity counts per feature table)
- [x] 4.5 Implement `GET /admin/analytics/payments?from=&to=` endpoint (count and sum grouped by status from transactions)
- [x] 4.6 Cache analytics query results in Redis with 60s TTL

## 5. Admin User Management API (Backend)

- [x] 5.1 Implement `GET /admin/users` endpoint with pagination and name/email search
- [x] 5.2 Implement `GET /admin/users/:id` endpoint returning user profile + payment history

## 6. Admin Campaign & Notifications API (Backend)

- [x] 6.1 Implement `POST /admin/campaigns` endpoint accepting payload (title, body, channel: push|email|both, segment: all|subscribers)
- [x] 6.2 Implement batch FCM dispatch in chunks of 500 with exponential backoff and per-token failure logging
- [x] 6.3 Implement email dispatch via existing template engine for campaign messages
- [x] 6.4 Extend Midtrans webhook handler (`POST /payment/webhook`) to send payment receipt push + email on `settlement` status
- [x] 6.5 Extend webhook handler to send payment failure push notification on `deny`/`expire` status

## 7. React App Bootstrap (Frontend)

- [x] 7.1 Initialize Vite + React + TypeScript project in `arunika_backoffice/`
- [x] 7.2 Install dependencies: `react-router-dom`, `@tanstack/react-query`, `antd`, `react-chartjs-2`, `chart.js`, `axios`
- [x] 7.3 Set up project folder structure: `src/pages`, `src/components`, `src/api`, `src/hooks`, `src/store`
- [x] 7.4 Configure environment variable for backend API base URL (`VITE_API_BASE_URL`)
- [x] 7.5 Set up Axios instance with base URL and request interceptor to attach admin JWT header
- [x] 7.6 Set up React Query client with default stale time and error handling

## 8. Admin Auth UI (Frontend)

- [x] 8.1 Create login page (`/login`) with email + password form using Ant Design Form
- [x] 8.2 Implement auth API calls: login, refresh, logout
- [x] 8.3 Store admin JWT in memory (not localStorage); use refresh token in httpOnly cookie pattern or localStorage with awareness
- [x] 8.4 Implement protected route wrapper that redirects unauthenticated users to `/login`
- [x] 8.5 Add logout button in sidebar/header that calls logout API and clears token

## 9. App Shell & Navigation (Frontend)

- [x] 9.1 Create main layout with Ant Design `Layout` + `Sider` sidebar navigation
- [x] 9.2 Add sidebar nav items: Dashboard, Content (sub-menu), Users, Payments, Campaigns
- [x] 9.3 Set up React Router routes for all sections under protected layout

## 10. Dashboard Page (Frontend)

- [x] 10.1 Create Dashboard page with 4 KPI summary cards: DAU (today), new users (today), payment success count, success rate %
- [x] 10.2 Add DAU line chart (last 30 days) using react-chartjs-2
- [x] 10.3 Add new users line chart (last 30 days)
- [x] 10.4 Add popular features bar chart (top 5)
- [x] 10.5 Add payment metrics bar chart (success vs failed last 30 days)
- [x] 10.6 Implement 30-second auto-refresh using React Query `refetchInterval`

## 11. Content Management Pages (Frontend)

- [x] 11.1 Create reusable `ContentTable` component with pagination, search, hidden badge, and action buttons (edit, toggle visibility, delete)
- [x] 11.2 Create Fairy Tales content page with list table and create/edit modal form
- [x] 11.3 Create AR Cards content page with list table and create/edit modal
- [x] 11.4 Create Tracing Items content page with list table and create/edit modal
- [x] 11.5 Create Counting Questions content page with list table and create/edit modal
- [x] 11.6 Create Badges content page with list table and create/edit modal
- [x] 11.7 Create Categories content page with list table and create/edit modal
- [x] 11.8 Implement visibility toggle (optimistic update) calling `PATCH /.../visibility`
- [x] 11.9 Implement soft-delete with confirmation dialog

## 12. User Management Page (Frontend)

- [x] 12.1 Create Users list page with paginated table and name/email search
- [x] 12.2 Create User Detail page showing profile info, subscription status, and payment history table

## 13. Payment Management Page (Frontend)

- [x] 13.1 Create Payments list page with paginated table, status filter, and date range picker
- [x] 13.2 Create Transaction Detail drawer/modal showing full payment info and raw webhook JSON
- [x] 13.3 Add payment summary stats bar at top of payments page (total, success, failed, pending)

## 14. Campaign Notifications Page (Frontend)

- [x] 14.1 Create Campaigns page with compose form: title, body, channel selector (push/email/both), segment selector (all/subscribers)
- [x] 14.2 Add confirmation dialog before dispatching campaign
- [x] 14.3 Display dispatch result summary (sent count, failed count) after submission

## 15. Integration & QA

- [ ] 15.1 Test all admin API endpoints with Postman or curl; verify mobile routes unaffected
- [ ] 15.2 Test content hide/show flow end-to-end: toggle in backoffice → verify mobile app response
- [ ] 15.3 Test payment receipt notification via Midtrans webhook simulator
- [ ] 15.4 Test campaign push dispatch with a real device token
- [ ] 15.5 Verify admin token cannot access mobile-only routes and mobile token cannot access admin routes
- [x] 15.6 Run build (`npm run build`) and verify no TypeScript errors
