## ADDED Requirements

### Requirement: Admin can view AR card categories
The system SHALL provide an AR Card Categories page listing all categories.

#### Scenario: Categories list loads
- **WHEN** admin navigates to the AR Card Categories page
- **THEN** the system SHALL fetch and display all categories in a table

### Requirement: Admin can add an AR card category
The system SHALL allow admins to create a new AR card category.

#### Scenario: Successful category creation
- **WHEN** admin submits the add category form with valid data
- **THEN** the system SHALL create the category and refresh the list

### Requirement: Admin can update an AR card category
The system SHALL allow admins to edit an existing AR card category.

#### Scenario: Successful category update
- **WHEN** admin opens the edit form for a category, changes data, and submits
- **THEN** the system SHALL update the category and reflect changes in the list

### Requirement: Admin can hide or show an AR card category
The system SHALL allow admins to toggle the visibility of an AR card category.

#### Scenario: Hide active category
- **WHEN** admin toggles visibility off for an active category
- **THEN** the system SHALL mark the category as hidden

#### Scenario: Show hidden category
- **WHEN** admin toggles visibility on for a hidden category
- **THEN** the system SHALL mark the category as visible

### Requirement: Admin can delete an AR card category
The system SHALL allow admins to permanently delete an AR card category.

#### Scenario: Successful deletion
- **WHEN** admin confirms deletion of a category
- **THEN** the system SHALL delete the category and remove it from the list
