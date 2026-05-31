## MODIFIED Requirements

### Requirement: AR card create/edit form
The system SHALL include all AR card fields in the create and edit form, including `image_url` and `printable_img`.

#### Scenario: Form renders all fields
- **WHEN** admin opens the AR card create or edit form
- **THEN** the form SHALL include fields for all AR card properties including `image_url` and `printable_img`

### Requirement: Dongeng entry includes page management
The system SHALL allow admins to manage pages of a dongeng directly from the dongeng management UI.

#### Scenario: Page management accessible from dongeng view
- **WHEN** admin opens or expands a dongeng entry
- **THEN** the system SHALL show a pages section with add, edit, and delete controls
