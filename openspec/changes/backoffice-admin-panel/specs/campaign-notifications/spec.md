## ADDED Requirements

### Requirement: Send push notification campaign
The system SHALL allow admins to compose and dispatch a push notification to all users or a filtered segment (e.g., active subscribers only).

#### Scenario: Send push to all users
- **WHEN** an admin fills in a campaign title and body and selects "All Users" then submits
- **THEN** the system dispatches FCM push notifications to all registered device tokens in batches of 500

#### Scenario: Send push to subscribers only
- **WHEN** an admin selects "Subscribers Only" as the target segment
- **THEN** only users with an active subscription receive the push notification

#### Scenario: Failed token handling
- **WHEN** a device token is invalid or unregistered during campaign dispatch
- **THEN** the system logs the failure per token and continues dispatching to remaining tokens

### Requirement: Send email notification campaign
The system SHALL allow admins to send an email campaign to users. Email sending SHALL use the existing template engine in the backend.

#### Scenario: Send email campaign
- **WHEN** an admin composes an email campaign with subject and body and submits
- **THEN** the system sends the email to the selected user segment via the configured SMTP service

### Requirement: Payment receipt notification
The system SHALL automatically send a push notification and/or email to a user when their payment is confirmed successful via the Midtrans webhook.

#### Scenario: Payment success receipt
- **WHEN** the Midtrans webhook delivers a `settlement` status event for a transaction
- **THEN** the system sends a push notification and email receipt to the paying user confirming their subscription activation

#### Scenario: Payment failure notification
- **WHEN** the Midtrans webhook delivers a `deny` or `expire` status event
- **THEN** the system sends a push notification informing the user their payment was unsuccessful
