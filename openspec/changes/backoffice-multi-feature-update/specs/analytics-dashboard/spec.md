## ADDED Requirements

### Requirement: Admin can view DAU analytics
The system SHALL provide a dedicated Analytics page accessible from the sidebar that displays Daily Active Users (DAU) data without errors.

#### Scenario: Page loads DAU chart successfully
- **WHEN** admin navigates to the Analytics page
- **THEN** the system SHALL fetch `/admin/analytics/dau` and render a chart with the results

#### Scenario: DAU endpoint returns 500
- **WHEN** the `/admin/analytics/dau` endpoint returns a 500 error
- **THEN** the system SHALL display an error message to the admin instead of crashing

### Requirement: Admin can view new-user analytics
The system SHALL display a new-users trend chart on the Analytics page.

#### Scenario: Page loads new-users chart successfully
- **WHEN** admin navigates to the Analytics page
- **THEN** the system SHALL fetch `/admin/analytics/new-users` and render a chart with the results

#### Scenario: New-users endpoint returns 500
- **WHEN** the `/admin/analytics/new-users` endpoint returns a 500 error
- **THEN** the system SHALL display an error message to the admin instead of crashing
