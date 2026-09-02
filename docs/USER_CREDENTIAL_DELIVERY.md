# User Credential Delivery via Email

## Overview

Technicians can now create new user accounts and automatically send login credentials via email. This feature streamlines the onboarding process for MSP staff and eliminates the need to manually share passwords.

## Backend Implementation

### Updated API Endpoint
**POST /api/users** (requires `PermManageUsers` permission)

Request body now supports:
```json
{
  "username": "john_tech",
  "password": "TempPassword123",
  "email": "john@techcompany.com",
  "role": "operator",
  "sendEmail": true
}
```

### Changes Made

**1. Database Schema Updates** (`internal/storage/sqlite.go`)
- Added `email TEXT` column to `users` table
- Updated `CreateUser()` function signature to accept email parameter

**2. Email Handler** (`internal/httpapi/server.go`)
- Modified `handleUsers()` POST handler to:
  - Accept email and sendEmail fields
  - Hash password securely with bcrypt
  - Create user in database with email
  - Send professional HTML email if sendEmail=true and SMTP configured
  
**3. Email Template**
- Professional HTML email template includes:
  - Formatted username and temporary password
  - User role assignment
  - Security warning to change password on first login
  - Branded GoMeshCentral header

### Email Sending Logic
```go
if body.SendEmail && body.Email != "" && s.emailService.IsConfigured() {
    // Sends email with credentials only if:
    // 1. sendEmail checkbox was checked
    // 2. Email field is not empty
    // 3. SMTP server is configured (email.IsConfigured() returns true)
}
```

## Frontend Implementation

### Users & Roles Page Updates (`web/src/App.jsx`)

**New Form Fields:**
- Email address input field
- "Send credentials via email" checkbox

**Updated Form State:**
```javascript
const [newUserForm, setNewUserForm] = useState({
  username: "",
  password: "",
  email: "",
  role: "viewer",
  sendEmail: false
});
```

**User Table Enhancements:**
- Added Email column displaying user's email
- Shows "-" if email is not set
- Maintains existing Username, Role, and Created columns

### User Interface Flow
1. Admin enters username, email, password, and selects role
2. Optionally checks "Send credentials via email" 
3. Clicks "Create User"
4. If email is enabled:
   - User receives email with login credentials
   - Admin sees success message in appStatus
5. User appears in the Users table with email address

## Configuration

### Email Service Setup
Credentials must be configured in your application settings:
- SMTP Host (e.g., smtp.gmail.com)
- SMTP Port (typically 587 or 465)
- SMTP Username
- SMTP Password
- From Address

Navigate to **Settings > Application** in the admin dashboard to configure email.

### Verification
The system checks `emailService.IsConfigured()` before sending emails. If SMTP is not configured:
- User creation still succeeds
- Email is not sent
- No error is displayed to admin

## Security Considerations

### Password Handling
1. **Generation**: Admin must create a secure temporary password
   - Recommendation: Use 12+ character passwords with mixed case, numbers, symbols
   - Example: `TempPass123!`

2. **Transmission**: Password is sent via email (use SMTP with TLS/SSL)
   - Ensure SMTP_OVER_TLS or STARTTLS is enabled
   - Never send unencrypted

3. **First Login**: User should:
   - Log in with provided credentials
   - Immediately change password
   - This is enforced if you add a force-change-password middleware

### Audit Trail
- User creation is logged in audit events
- Email sending attempts are not currently logged separately
- Consider monitoring SMTP server logs for delivery confirmation

## Database Migration

The migration is automatic and non-breaking:
- Adds `email TEXT` column to existing `users` table
- Existing users will have NULL email values
- No data loss occurs

If manually running migrations:
```sql
ALTER TABLE users ADD COLUMN email TEXT;
```

## API Response Codes

- **201 Created**: User successfully created (regardless of email send status)
- **400 Bad Request**: Missing required fields (username, password, role)
- **409 Conflict**: Username already exists
- **500 Internal Server Error**: Password hashing or database error

## Future Enhancements

Potential improvements to this feature:

1. **Temporary Password Generation**
   - Auto-generate random secure passwords
   - Display in UI for admin to copy/share

2. **Password Change Enforcement**
   - Add `force_password_change` flag to User model
   - Redirect user to change password on first login
   - Track when password was last changed

3. **Email Templates**
   - Customizable HTML email templates
   - Support for organization branding
   - Multiple language support

4. **Bulk User Import**
   - CSV upload for creating multiple users
   - Batch email delivery
   - Import status tracking

5. **Email Verification**
   - Verify email address before user activation
   - Send confirmation link
   - Prevent typos in email addresses

6. **MFA Enrollment**
   - Send MFA setup link in welcome email
   - Include QR code for authenticator apps
   - Improve account security from day one

## Testing the Feature

### Manual Test Steps

1. **Via Admin Dashboard:**
   - Navigate to Users & Roles page
   - Fill in: username, email, password, role
   - Check "Send credentials via email"
   - Click "Create User"
   - Check recipient's inbox for welcome email

2. **Via API (with curl):**
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "SecurePassword123",
    "email": "newuser@company.com",
    "role": "operator",
    "sendEmail": true
  }'
```

3. **Email Configuration Validation:**
   - Check Settings > Application
   - Verify SMTP host, port, username, password
   - Send test email through settings page

## Troubleshooting

### Email Not Received
1. **Check SMTP Configuration**
   - Verify credentials in Settings > Application
   - Ensure SMTP server is accessible from network
   - Check firewall rules for SMTP port

2. **Check Server Logs**
   - Look for email service initialization errors
   - Verify sendEmail=true in the request
   - Confirm email field is populated

3. **Gmail/Office 365 Common Issues**
   - Gmail: Enable "Less secure app access" or use app-specific password
   - Office 365: Use TLS on port 587
   - Both: Verify relay IP is whitelisted

### User Created But No Email
- This is expected if SMTP is not configured
- User can still log in with provided credentials
- Configure email later and resend manually if needed

### Database Errors
- If migration fails, check SQLite file permissions
- Ensure users table exists before running app
- Run migrations manually if needed: `go run . migrate`

## Integration with Other Systems

This feature integrates with:
- **Authentication Service** (`internal/auth`) - JWT token generation for new users
- **Email Service** (`internal/email`) - SMTP delivery configuration
- **Database** (`internal/storage`) - User persistence and schema
- **Audit Logging** - User creation events tracked

## API Version Compatibility

Current implementation is v1 (no versioning prefix). If API changes:
- Version increment required
- Backward compatibility maintained for sendEmail field
- Email field optional for existing integrations
