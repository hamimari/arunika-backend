## ADDED Requirements

### Requirement: Admin login
The system SHALL authenticate admin users via email and bcrypt-hashed password, issuing a signed JWT with `role: admin` claim. Admin credentials are stored in a separate `admin_users` table and SHALL NOT share tokens with mobile app users.

#### Scenario: Successful admin login
- **WHEN** an admin submits valid email and password to `POST /admin/auth/login`
- **THEN** the system returns a JWT access token and a refresh token with HTTP 200

#### Scenario: Invalid credentials
- **WHEN** an admin submits an incorrect password or unknown email
- **THEN** the system returns HTTP 401 with an error message; no token is issued

#### Scenario: Mobile user token rejected on admin routes
- **WHEN** a request is made to any `/admin/*` route with a mobile user JWT
- **THEN** the system returns HTTP 403

### Requirement: Admin session management
The system SHALL support token refresh and logout for admin sessions.

#### Scenario: Refresh admin token
- **WHEN** an admin submits a valid refresh token to `POST /admin/auth/refresh`
- **THEN** the system issues a new access token

#### Scenario: Admin logout
- **WHEN** an admin sends `POST /admin/auth/logout` with a valid token
- **THEN** the token is invalidated in Redis and subsequent requests with it return HTTP 401
