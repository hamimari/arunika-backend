## ADDED Requirements

### Requirement: AR card form includes image_url field
The system SHALL include an `image_url` text input in the AR card create and edit form.

#### Scenario: Admin creates AR card with image_url
- **WHEN** admin fills in the AR card form and provides a value for `image_url`
- **THEN** the system SHALL include `image_url` in the create request payload

#### Scenario: Admin edits AR card with existing image_url
- **WHEN** admin opens the edit modal for an AR card that has an `image_url`
- **THEN** the system SHALL pre-populate the `image_url` field with the existing value

### Requirement: AR card form includes printable_img field
The system SHALL include a `printable_img` text input in the AR card create and edit form.

#### Scenario: Admin creates AR card with printable_img
- **WHEN** admin fills in the AR card form and provides a value for `printable_img`
- **THEN** the system SHALL include `printable_img` in the create request payload

#### Scenario: Admin edits AR card with existing printable_img
- **WHEN** admin opens the edit modal for an AR card that has a `printable_img`
- **THEN** the system SHALL pre-populate the `printable_img` field with the existing value
