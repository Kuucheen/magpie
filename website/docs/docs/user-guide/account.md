# Account and Authentication

## Register

`POST /api/register`

- validates email format
- requires password length >= 8
- first user gets admin role

## Login

`POST /api/login`

- returns JWT token and role

## Session check

`GET /api/checkLogin`

- requires valid bearer token

## Change password

`POST /api/changePassword`

- requires old password match

## Delete account

`POST /api/deleteAccount`

- requires current password
- removes user relationships and orphaned resources where applicable
