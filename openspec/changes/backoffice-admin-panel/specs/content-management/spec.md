## ADDED Requirements

### Requirement: List content items
The system SHALL allow admins to list all items for each content type: fairy tales, AR cards, tracing items, counting questions, badges, and categories. Lists SHALL support pagination and search by title/name.

#### Scenario: List fairy tales
- **WHEN** an admin navigates to the Fairy Tales content section
- **THEN** a paginated table shows all fairy tales including hidden ones, with columns: title, category, hidden status, created date

#### Scenario: Search content
- **WHEN** an admin enters a search term in the content list
- **THEN** the table filters to show only items whose title/name contains the search term

### Requirement: Create content item
The system SHALL allow admins to create new items for each content type via a form that mirrors all required fields.

#### Scenario: Create fairy tale
- **WHEN** an admin submits a valid create-fairy-tale form
- **THEN** the new fairy tale is saved to the database and appears in the list

#### Scenario: Validation failure on create
- **WHEN** an admin submits a form with missing required fields
- **THEN** the system returns validation errors and does not save the item

### Requirement: Update content item
The system SHALL allow admins to edit existing content items and save changes.

#### Scenario: Update fairy tale
- **WHEN** an admin edits and submits a fairy tale
- **THEN** the updated data is persisted and reflected in the list

### Requirement: Delete content item
The system SHALL allow admins to soft-delete content items (setting `deleted_at`). Deleted items SHALL NOT appear in mobile app responses.

#### Scenario: Delete content item
- **WHEN** an admin confirms deletion of a content item
- **THEN** the item's `deleted_at` is set to the current timestamp and it no longer appears in mobile API responses

### Requirement: Toggle content visibility
The system SHALL allow admins to hide or show content items. Hidden items SHALL NOT appear in mobile app list/detail responses.

#### Scenario: Hide a fairy tale
- **WHEN** an admin toggles a fairy tale to hidden
- **THEN** the fairy tale's `hidden` flag is set to `true` and mobile clients no longer receive it in list responses

#### Scenario: Show a hidden fairy tale
- **WHEN** an admin toggles a hidden fairy tale back to visible
- **THEN** the fairy tale's `hidden` flag is set to `false` and mobile clients receive it again
