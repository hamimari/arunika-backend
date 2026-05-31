## ADDED Requirements

### Requirement: List users
The system SHALL allow admins to view a paginated list of all registered users with name, email, registration date, and subscription status.

#### Scenario: View user list
- **WHEN** an admin navigates to the Users section
- **THEN** a paginated table shows all users sorted by registration date descending

#### Scenario: Search users
- **WHEN** an admin searches by name or email
- **THEN** the list filters to matching users

### Requirement: View user detail
The system SHALL display a user's full profile including personal info, subscription status, and payment history.

#### Scenario: View user profile
- **WHEN** an admin clicks on a user in the list
- **THEN** the system shows name, email, date of birth, subscription expiry, and a list of all payment transactions for that user

### Requirement: View user payment history
The system SHALL display all payment transactions for a specific user including amount, status (success/pending/failed), and Midtrans order ID.

#### Scenario: Payment history on user detail
- **WHEN** an admin views a user's detail page
- **THEN** a table shows all transactions for that user with: order ID, amount, status, and transaction date
