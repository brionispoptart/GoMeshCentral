# Quick Start: User Credential Delivery via Email

## Feature Summary

Technicians can now **create user accounts and automatically send login credentials via email**. This streamlines staff onboarding while maintaining security through temporary passwords.

## How It Works

### For Technicians (Admin Dashboard)

1. Navigate to **Settings → Users & Roles** in the admin dashboard
2. Fill in the user creation form:
   - **Username**: Unique login name (e.g., `john_smith`)
   - **Email**: User's email address (e.g., `john@techcompany.com`)
   - **Password**: Initial temporary password (e.g., `TempPass123!`)
   - **Role**: Select from viewer, operator, or admin
3. Check **"Send credentials via email"** if SMTP is configured
4. Click **"Create User"**
5. User receives email with login details

### Email Template

Recipients get a professional email containing:
- Username and temporary password
- Role assignment
- Security notice to change password immediately upon login
- Branded GoMeshCentral header

Example:
```
Welcome to GoMeshCentral

Your account has been created. Here are your login credentials:

Username: john_smith
Password: TempPass123!
Role: operator

⚠️ Important: Please change your password immediately upon first login.
```

## Configuration

### Enable Email Delivery

Email sending requires SMTP configuration. Configure in **Settings → Application**:

| Setting | Example | Notes |
|---------|---------|-------|
| SMTP Host | smtp.gmail.com | Your email provider's SMTP server |
| SMTP Port | 587 | 587 for TLS, 465 for SSL |
| SMTP Username | your-email@gmail.com | Account to send emails from |
| SMTP Password | app-specific-password | Use app-specific password for Gmail |
| From Address | noreply@techcompany.com | Display address in sent emails |

### Provider Guides

**Gmail:**
1. Enable 2-Factor Authentication
2. Create App Password at https://myaccount.google.com/apppasswords
3. Use app-specific password in SMTP Password field
4. Port: 587, TLS enabled

**Office 365:**
1. Use your Office 365 email as SMTP Username
2. Use your Office 365 password
3. SMTP Host: smtp.office365.com
4. Port: 587, TLS required

**Custom SMTP Server:**
1. Contact your email administrator for credentials
2. Use provided SMTP server details

## API Reference

### Create User with Email

**Endpoint:** `POST /api/users`

**Authentication:** Required (admin role)

**Request Body:**
```json
{
  "username": "newuser",
  "password": "SecureTemporaryPassword123",
  "email": "newuser@company.com",
  "role": "operator",
  "sendEmail": true
}
```

**Parameters:**
- `username` (string, required): Unique username
- `password` (string, required): Initial temporary password
- `email` (string, optional): User's email address
- `role` (string, required): One of `viewer`, `operator`, `admin`
- `sendEmail` (boolean, optional): Send credentials via email (default: false)

**Response:**
- `201 Created`: User created successfully
- `400 Bad Request`: Missing required fields
- `409 Conflict`: Username already exists
- `500 Internal Server Error`: Server error

**Example:**
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "sarah_tech",
    "password": "InitialPass789!",
    "email": "sarah@company.com",
    "role": "operator",
    "sendEmail": true
  }'
```

## Database Schema

### Users Table Updates

New column added to track user emails:

```sql
ALTER TABLE users ADD COLUMN email TEXT;
```

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PRIMARY KEY | Auto-increment |
| username | TEXT UNIQUE | Unique identifier |
| email | TEXT | User's email address |
| password_hash | TEXT | Bcrypt hashed password |
| role | TEXT | User role (viewer, operator, admin) |
| org_id | TEXT | Organization ID |
| created_at | TEXT | Creation timestamp |

## Security Best Practices

### Password Guidelines

1. **Temporary Password Requirements:**
   - Minimum 12 characters
   - Mix uppercase and lowercase letters
   - Include numbers and special characters
   - Example: `SecureTemp#2024`

2. **Password Transmission:**
   - Always use SMTP with TLS/SSL encryption
   - Never send passwords over unencrypted channels
   - Verify SMTP server certificate validity

3. **First Login:**
   - Users should change password immediately
   - Consider implementing password change enforcement
   - Log password change events

### Email Security

1. **Verify Email Addresses:**
   - Double-check email addresses before sending
   - Email delivery failures won't block user creation
   - Consider sending test email first

2. **Track Deliveries:**
   - Monitor SMTP server logs
   - Check email bounce reports
   - Resend credentials if delivery fails

3. **Audit Trail:**
   - User creation events are logged
   - Technician who created user is recorded
   - Review audit events in admin dashboard

## Troubleshooting

### Email Not Received

**Check 1: SMTP Configuration**
```powershell
# Test SMTP connection
telnet smtp.gmail.com 587
```

**Check 2: Firewall Rules**
- Verify outbound SMTP port is open
- Check network firewall settings
- Confirm proxy doesn't block SMTP

**Check 3: Email Provider**
- Gmail: Verify "Less Secure Apps" is enabled or use app password
- Office 365: Verify TLS/SSL settings
- Custom: Verify IP is whitelisted

**Check 4: Application Logs**
- Look for SMTP errors in server output
- Check email service initialization
- Verify emailService.IsConfigured() returns true

### User Created, Email Not Sent

This is normal if SMTP is not configured. The system will:
1. Create the user successfully ✓
2. Skip email sending (not an error) ✓
3. Display success message to admin ✓

To enable later:
1. Configure SMTP in Settings
2. Share credentials manually with user
3. User can reset password if needed

### Email Stuck in Spam

**Common Solutions:**
1. Add "noreply@yourdomain.com" to email whitelist
2. Configure SPF/DKIM/DMARC records
3. Use company email domain instead of Gmail
4. Request IT to check spam filter rules

## Changelog

### Version 1.0 (Current)

**Features Added:**
- ✅ User creation with email field storage
- ✅ Optional email credential delivery
- ✅ Professional HTML email template
- ✅ SMTP configuration in settings
- ✅ Admin dashboard UI for email field
- ✅ Email column in users table

**Backend Changes:**
- Updated `users` table schema with email column
- Modified `CreateUser()` API to accept email
- Added `sendEmail` request parameter
- Implemented HTML email template rendering
- SMTP service integration

**Frontend Changes:**
- Added email input field to user creation form
- Added "Send credentials via email" checkbox
- Email column in users table display
- Form validation for email format

**Testing:**
- Unit tests for email credential delivery
- Database schema migration tested
- API endpoint validation

## Support & Documentation

For additional help:
- Review [User Credential Delivery Documentation](./USER_CREDENTIAL_DELIVERY.md)
- Check Server Logs: `logs-server.txt`
- Test Endpoint: `/api/users` with admin token
- Database: Check `users.email` column is present

## Future Roadmap

Planned enhancements:
- [ ] Auto-generate secure random passwords
- [ ] Force password change on first login
- [ ] Bulk user import from CSV
- [ ] Email verification before account activation
- [ ] MFA enrollment via email link
- [ ] Customizable email templates
- [ ] Email delivery status tracking
- [ ] Resend credentials option in admin dashboard
