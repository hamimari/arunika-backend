## ADDED Requirements

### Requirement: Daily active users metric
The system SHALL track and display the count of unique users who logged in on each calendar day (UTC), derived from a `user_sessions` table.

#### Scenario: View DAU on dashboard
- **WHEN** an admin loads the dashboard
- **THEN** the system displays a line chart of DAU for the last 30 days

### Requirement: New user signups metric
The system SHALL display the count of new user registrations per day for the last 30 days.

#### Scenario: View new users chart
- **WHEN** an admin loads the dashboard
- **THEN** a chart shows daily new signup counts alongside a total for the selected period

### Requirement: Popular features metric
The system SHALL rank features (fairy tales, AR, tracing, counting, growth) by total interaction count and display the top 5.

#### Scenario: View popular features
- **WHEN** an admin loads the dashboard
- **THEN** a bar chart displays features ordered by usage count descending

### Requirement: Payment metrics
The system SHALL display the total number of payment attempts, number of successful payments, and number of failed payments for the current day and last 30 days.

#### Scenario: View payment summary
- **WHEN** an admin loads the dashboard
- **THEN** summary cards show: total transactions, successful count, failed count, and success rate percentage

### Requirement: Dashboard data refresh
The system SHALL refresh dashboard metrics every 30 seconds without requiring a manual page reload.

#### Scenario: Auto-refresh
- **WHEN** 30 seconds have elapsed since the last data fetch
- **THEN** all dashboard KPI components re-fetch data from the backend silently
