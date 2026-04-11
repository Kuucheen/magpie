# Account and Authentication

## Register

`POST /api/register`

- validates email format
- normalizes email input (trimmed, lowercase)
- requires a strong password:
  - at least 12 characters
  - at least one uppercase letter
  - at least one lowercase letter
  - at least one number
  - no whitespace
- first user gets admin role

## Login

`POST /api/login`

- returns JWT token and role
- email lookup is case-insensitive
- protected by request and failed-login rate limiting

## Forgot password

`POST /api/forgotPassword`

- public endpoint
- always returns a generic success message when the request is accepted
- sends a reset email only if the account exists and outbound email is configured
- per-account reset-email requests are throttled by default to 1 request per minute

## Reset password

`POST /api/resetPassword`

- public endpoint
- requires a valid reset token and a password that meets the same policy as registration
- reset tokens are single-use and expire automatically
- successful reset removes all outstanding reset tokens for that user
- successful reset also revokes active sessions under normal revocation-store operation
- a confirmation email is queued after a successful reset

## Session check

`GET /api/checkLogin`

- requires valid bearer token

## Change password

`POST /api/changePassword`

- requires old password match
- requires the same strong password policy used by registration and password reset
- successful change revokes active sessions

## Delete account

`POST /api/deleteAccount`

- requires current password
- removes user relationships and orphaned resources where applicable
