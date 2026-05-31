## ADDED Requirements

### Requirement: Admin can manage pages within a dongeng entry
The system SHALL allow admins to add, update, and delete individual pages within an existing dongeng (fairy tale).

#### Scenario: Admin views pages of a dongeng
- **WHEN** admin opens a dongeng entry in the management UI
- **THEN** the system SHALL display all pages belonging to that dongeng

#### Scenario: Admin adds a new page
- **WHEN** admin submits the add page form within a dongeng
- **THEN** the system SHALL create the page and append it to the dongeng's page list

#### Scenario: Admin updates an existing page
- **WHEN** admin edits a page within a dongeng and submits
- **THEN** the system SHALL update the page and reflect changes in the list

#### Scenario: Admin deletes a page
- **WHEN** admin confirms deletion of a page within a dongeng
- **THEN** the system SHALL delete the page and remove it from the list
