## ADDED Requirements

### Requirement: List all transactions
The system SHALL allow admins to view a paginated list of all payment transactions across all users.

#### Scenario: View transaction list
- **WHEN** an admin navigates to Payments
- **THEN** a table shows all transactions with: user name, order ID, amount, status, and date

#### Scenario: Filter by status
- **WHEN** an admin applies a status filter (success / pending / failed)
- **THEN** only transactions matching that status are shown

### Requirement: View transaction detail
The system SHALL display full payment detail for a single transaction including Midtrans response payload.

#### Scenario: Open transaction detail
- **WHEN** an admin clicks a transaction row
- **THEN** a detail panel shows: user info, order ID, amount, payment method, Midtrans status, and raw webhook response (JSON)

### Requirement: Payment summary statistics
The system SHALL provide a summary view of payment counts and amounts grouped by status for any selectable date range.

#### Scenario: View payment summary for date range
- **WHEN** an admin selects a date range on the payments page
- **THEN** the system displays: total transactions, successful total (count + amount), failed count, pending count
