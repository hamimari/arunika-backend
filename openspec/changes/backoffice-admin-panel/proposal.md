## Why

Arunika's mobile app has grown to include multiple features (fairy tales, AR, tracing, counting, badges, payments, notifications, growth tracking) but there is no centralized admin interface to manage content, monitor users, handle payments, or run campaigns. A backoffice admin panel will give the operations team full visibility and control over the platform without requiring direct database or API access.

## What Changes

- New standalone React web application at `/Users/hamim.tohari/Project/arunika_backoffice`
- New admin-only API endpoints added to the existing Go backend (or a new admin service if scope requires)
- Dashboard with real-time KPIs: daily active users, new user signups, popular features, payment counts, and payment success/failure rates
- Content management for all mobile app features: fairy tales (dongeng), AR cards, tracing items, counting questions, badges, categories
- Content visibility controls (show/hide) and full CRUD operations per content type
- User management with ability to view profiles and payment histories
- Payment detail viewer with transaction statuses from Midtrans webhook data
- Push notification and email campaign system leveraging the existing notification service
- Payment receipt notifications triggered by Midtrans webhook events
- Admin authentication separate from the mobile app's user auth flow

## Capabilities

### New Capabilities

- `admin-auth`: Admin login and session management (separate from mobile user auth)
- `dashboard`: KPI monitoring — DAU, new users, popular features, payment metrics
- `content-management`: CRUD + visibility toggle for fairy tales, AR cards, tracing items, counting questions, badges, categories
- `user-management`: View user list, profiles, and subscription status
- `payment-management`: View all transactions, payment details, success/failure breakdown
- `campaign-notifications`: Send push or email notifications to user segments for campaigns
- `admin-api`: New backend routes under `/admin` prefix with admin role middleware

### Modified Capabilities

- `payment`: Webhook handler extended to emit events for admin dashboard payment metrics

## Impact

- **Backend** (`/Users/hamim.tohari/Project/arunika backend`): New `/admin` route group with auth, CRUD endpoints for all content types, analytics aggregation endpoints, and campaign notification dispatch
- **Frontend**: New React app at `arunika_backoffice` (this repo), communicating with the backend admin API
- **Database**: New `admin_users` table; analytics may require aggregation views or a lightweight metrics store
- **Notifications**: Existing FCM/email notification service reused for campaign sending
- **Auth**: New admin JWT role or separate admin token issuance, not shared with mobile user tokens
