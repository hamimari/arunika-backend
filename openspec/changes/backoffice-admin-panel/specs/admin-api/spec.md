## ADDED Requirements

### Requirement: Admin route group with authentication middleware
The backend SHALL expose all admin endpoints under `/admin` prefix. Every `/admin/*` route (except `/admin/auth/login`) SHALL require a valid admin JWT validated by `AdminAuthMiddleware`. Requests with mobile user tokens SHALL be rejected with HTTP 403.

#### Scenario: Unauthenticated admin request
- **WHEN** a request to `/admin/*` carries no Authorization header
- **THEN** the server returns HTTP 401

#### Scenario: Mobile token on admin route
- **WHEN** a request to `/admin/*` carries a JWT with `role: user`
- **THEN** the server returns HTTP 403

### Requirement: Admin content CRUD endpoints
The backend SHALL provide RESTful CRUD endpoints for each content type under `/admin/content/{type}`.

#### Scenario: Create content via admin API
- **WHEN** admin sends `POST /admin/content/fairy-tales` with valid payload
- **THEN** the server creates the record and returns it with HTTP 201

#### Scenario: Toggle visibility via admin API
- **WHEN** admin sends `PATCH /admin/content/fairy-tales/:id/visibility` with `{"hidden": true}`
- **THEN** the server updates the `hidden` flag and returns HTTP 200

### Requirement: Admin analytics endpoints
The backend SHALL provide aggregated analytics endpoints for the dashboard.

#### Scenario: Get DAU data
- **WHEN** admin sends `GET /admin/analytics/dau?days=30`
- **THEN** the server returns an array of `{date, count}` objects for the last 30 days

#### Scenario: Get payment metrics
- **WHEN** admin sends `GET /admin/analytics/payments?from=&to=`
- **THEN** the server returns counts and totals grouped by status

### Requirement: Admin campaign dispatch endpoint
The backend SHALL expose `POST /admin/campaigns` that accepts a notification payload and user segment, dispatching push and/or email notifications.

#### Scenario: Dispatch campaign
- **WHEN** admin sends `POST /admin/campaigns` with title, body, channel (push|email|both), and segment
- **THEN** the server queues and dispatches notifications, returning a job summary with HTTP 202
