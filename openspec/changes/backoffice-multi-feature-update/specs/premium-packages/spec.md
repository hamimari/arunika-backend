## MODIFIED Requirements

### Requirement: Premium packages list shows all packages
The system SHALL display all premium packages to admins regardless of their active/inactive status.

#### Scenario: Inactive packages are visible
- **WHEN** admin navigates to the Premium Packages page
- **THEN** the system SHALL show both active and inactive packages in the list

#### Scenario: Inactive packages are visually distinguished
- **WHEN** an inactive package appears in the list
- **THEN** the system SHALL visually indicate its inactive status (e.g., greyed row or "Inactive" badge)
