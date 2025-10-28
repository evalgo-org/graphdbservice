# User Management Architecture - Visual Overview

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         GraphDB Service                              │
│                                                                       │
│  ┌──────────────┐        ┌──────────────┐       ┌──────────────┐   │
│  │   Browser    │        │  API Client  │       │  Admin UI    │   │
│  │   (Web UI)   │        │ (curl/REST)  │       │              │   │
│  └──────┬───────┘        └──────┬───────┘       └──────┬───────┘   │
│         │                       │                       │            │
│         │ Cookie: session=JWT   │ Header: x-api-key    │            │
│         ├───────────────────────┼───────────────────────┤            │
│         │                       │                       │            │
│         ▼                       ▼                       ▼            │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                Echo HTTP Server (Port 8080)                   │   │
│  │                                                                │   │
│  │  ┌────────────────────┐      ┌─────────────────────────┐    │   │
│  │  │ Auth Middleware    │      │ API Key Middleware      │    │   │
│  │  │ - Validate JWT     │      │ - Validate x-api-key    │    │   │
│  │  │ - Extract claims   │      │ - Rate limiting         │    │   │
│  │  │ - Check expiration │      │                         │    │   │
│  │  └─────────┬──────────┘      └───────────┬─────────────┘    │   │
│  │            │                              │                  │   │
│  │            ▼                              ▼                  │   │
│  │  ┌─────────────────┐          ┌──────────────────────┐     │   │
│  │  │  UI Handlers    │          │  API Handlers        │     │   │
│  │  │  - /            │          │  - /v1/api/action    │     │   │
│  │  │  - /ui/execute  │          │  - processTask()     │     │   │
│  │  │  - /login       │          │                      │     │   │
│  │  │  - /logout      │          │                      │     │   │
│  │  └────────┬────────┘          └──────────┬───────────┘     │   │
│  │           │                              │                  │   │
│  │           └──────────────┬───────────────┘                  │   │
│  │                          │                                  │   │
│  └──────────────────────────┼──────────────────────────────────┘   │
│                             │                                       │
│                             ▼                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                  Authentication Layer                         │  │
│  │                                                                │  │
│  │  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │  │
│  │  │   User      │  │   Session    │  │   Audit          │   │  │
│  │  │   Manager   │  │   Manager    │  │   Logger         │   │  │
│  │  │             │  │              │  │                  │   │  │
│  │  │ - Login     │  │ - JWT Gen    │  │ - Log actions   │   │  │
│  │  │ - Logout    │  │ - Validate   │  │ - Rotate logs   │   │  │
│  │  │ - CRUD      │  │ - Expire     │  │ - Retention     │   │  │
│  │  │ - Roles     │  │              │  │                  │   │  │
│  │  └──────┬──────┘  └──────┬───────┘  └─────────┬────────┘   │  │
│  │         │                │                     │            │  │
│  └─────────┼────────────────┼─────────────────────┼────────────┘  │
│            │                │                     │               │
│            ▼                ▼                     ▼               │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │              Filesystem Storage Layer                         │ │
│  │                                                                │ │
│  │  /data/                                                       │ │
│  │    ├── users/                                                 │ │
│  │    │   ├── users.json          ◄── Encrypted user database  │ │
│  │    │   ├── users.json.backup   ◄── Automatic backup         │ │
│  │    │   └── .users.lock          ◄── File lock (concurrency) │ │
│  │    │                                                          │ │
│  │    ├── sessions/                                              │ │
│  │    │   ├── sessions.json        ◄── Active sessions (opt)    │ │
│  │    │   └── cleanup.log          ◄── Cleanup history         │ │
│  │    │                                                          │ │
│  │    └── audit/                                                 │ │
│  │        └── 2025-10/                                           │ │
│  │            ├── 2025-10-28.log  ◄── Daily audit log (JSONL)  │ │
│  │            └── 2025-10-29.log                                │ │
│  │                                                                │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

## Authentication Flow Diagram

```
┌─────────┐                                    ┌──────────────────┐
│ Browser │                                    │     Server       │
└────┬────┘                                    └────────┬─────────┘
     │                                                  │
     │ 1. GET /                                        │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │                           2. No session cookie  │
     │                              → Check auth mode  │
     │                              → AUTH_MODE=rbac   │
     │                                                  │
     │ 3. Redirect 302 → /login                        │
     │<────────────────────────────────────────────────┤
     │                                                  │
     │ 4. GET /login                                   │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │ 5. ← Login Form HTML                            │
     │<────────────────────────────────────────────────┤
     │                                                  │
     │ 6. POST /auth/login                             │
     │    {username: "admin", password: "****"}        │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │                              7. Validate creds  │
     │                                 ├─ Load users.json
     │                                 ├─ Check rate limit
     │                                 ├─ bcrypt.Compare()
     │                                 ├─ Generate JWT
     │                                 ├─ Update last_login
     │                                 └─ Log audit entry
     │                                                  │
     │ 8. Set-Cookie: session=<JWT_TOKEN>              │
     │    HttpOnly; Secure; SameSite=Strict            │
     │    Redirect 302 → /                             │
     │<────────────────────────────────────────────────┤
     │                                                  │
     │ 9. GET /                                        │
     │    Cookie: session=<JWT_TOKEN>                  │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │                            10. Validate JWT     │
     │                                ├─ Parse token   │
     │                                ├─ Verify signature
     │                                ├─ Check expiration
     │                                └─ Extract claims │
     │                                   {user_id, role}│
     │                                                  │
     │ 11. ← Dashboard HTML (with username in navbar)  │
     │<────────────────────────────────────────────────┤
     │                                                  │
     │ 12. POST /ui/execute                            │
     │     Cookie: session=<JWT_TOKEN>                 │
     │     {task_json: "..."}                          │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │                            13. Validate JWT     │
     │                                Execute task     │
     │                                Log audit entry  │
     │                                                  │
     │ 14. ← Task Results HTML                         │
     │<────────────────────────────────────────────────┤
     │                                                  │
     │ ... time passes (1 hour) ...                    │
     │                                                  │
     │ 15. GET /ui/execute                             │
     │     Cookie: session=<JWT_TOKEN>                 │
     ├────────────────────────────────────────────────>│
     │                                                  │
     │                            16. Validate JWT     │
     │                                → Token EXPIRED  │
     │                                                  │
     │ 17. Clear cookie, Redirect 302 → /login         │
     │<────────────────────────────────────────────────┤
     │                                                  │
```

## User Management Flow (Admin)

```
┌──────────┐                                  ┌────────────────┐
│  Admin   │                                  │    Server      │
└────┬─────┘                                  └────────┬───────┘
     │                                                 │
     │ 1. GET /admin/users                            │
     │    Cookie: session=<JWT_ADMIN>                 │
     ├───────────────────────────────────────────────>│
     │                                                 │
     │                          2. Validate JWT       │
     │                             Check role="admin" │
     │                             Load users.json    │
     │                                                 │
     │ 3. ← User List HTML                            │
     │    [admin, user1, user2, ...]                  │
     │<───────────────────────────────────────────────┤
     │                                                 │
     │ 4. POST /admin/users                           │
     │    {username: "newuser",                       │
     │     password: "SecurePass123!",                │
     │     role: "user",                              │
     │     email: "new@example.com"}                  │
     ├───────────────────────────────────────────────>│
     │                                                 │
     │                          5. Validate request   │
     │                             ├─ Check unique username
     │                             ├─ Validate password
     │                             ├─ Hash password (bcrypt)
     │                             ├─ Create User object
     │                             ├─ Lock users.json
     │                             ├─ Add to database
     │                             ├─ Save atomically
     │                             ├─ Unlock
     │                             └─ Log audit entry
     │                                                 │
     │ 6. ← Success (new user row HTML)               │
     │<───────────────────────────────────────────────┤
     │                                                 │
     │ 7. DELETE /admin/users/user2                   │
     ├───────────────────────────────────────────────>│
     │                                                 │
     │                          8. Validate request   │
     │                             ├─ Check admin role
     │                             ├─ Prevent self-delete
     │                             ├─ Lock users.json
     │                             ├─ Remove user
     │                             ├─ Save atomically
     │                             ├─ Unlock
     │                             └─ Log audit entry
     │                                                 │
     │ 9. ← Success                                   │
     │<───────────────────────────────────────────────┤
     │                                                 │
```

## Data Flow - users.json Structure

```
┌────────────────────────────────────────────────────────────────┐
│ users.json (File: /data/users/users.json)                     │
├────────────────────────────────────────────────────────────────┤
│ {                                                              │
│   "version": "1.0.0",                                          │
│   "users": {                                                   │
│     "admin": {                                                 │
│       "id": "uuid-1234",                                       │
│       "username": "admin",                                     │
│       "email": "admin@example.com",                            │
│       "password_hash": "$2a$10$N9qo8uLO...",  ◄── bcrypt     │
│       "role": "admin",                         ◄── RBAC       │
│       "created_at": "2025-10-28T10:00:00Z",                   │
│       "updated_at": "2025-10-28T12:00:00Z",                   │
│       "last_login_at": "2025-10-28T12:30:00Z", ◄── Tracking   │
│       "failed_logins": 0,                      ◄── Rate limit │
│       "locked": false,                         ◄── Security   │
│       "must_change_password": false                           │
│     },                                                         │
│     "user1": { /* ... */ }                                     │
│   },                                                           │
│   "updated_at": "2025-10-28T12:00:00Z"                        │
│ }                                                              │
└────────────────────────────────────────────────────────────────┘
          │
          ├─── Read by: Login handler, User management
          ├─── Write by: Create/Update/Delete user operations
          └─── Protected: File permissions 0600, file locking
```

## JWT Token Structure

```
┌──────────────────────────────────────────────────────────────────┐
│ JWT Token (Stored in HTTP-only cookie)                          │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Header:                                                         │
│  {                                                               │
│    "alg": "HS256",        ◄── HMAC SHA256 signing              │
│    "typ": "JWT"                                                  │
│  }                                                               │
│                                                                  │
│  Payload (Claims):                                               │
│  {                                                               │
│    "user_id": "uuid-1234",     ◄── Unique identifier           │
│    "username": "admin",        ◄── Display name                │
│    "role": "admin",            ◄── Authorization               │
│    "exp": 1698504000,          ◄── Expiration (Unix timestamp) │
│    "iat": 1698500400,          ◄── Issued at                   │
│    "iss": "graphservice"       ◄── Issuer                      │
│  }                                                               │
│                                                                  │
│  Signature:                                                      │
│  HMACSHA256(                                                     │
│    base64(header) + "." + base64(payload),                      │
│    JWT_SECRET                   ◄── Secret key (env var)       │
│  )                                                               │
│                                                                  │
│  Final Token:                                                    │
│  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.                          │
│  eyJ1c2VyX2lkIjoidXVpZC0xMjM0IiwidXNlcm5hbWUiOi...             │
│  .SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c                  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
         │
         ├─── Stored: HTTP-only, Secure, SameSite=Strict cookie
         ├─── Validated: Every protected route request
         ├─── Expires: Configurable (default 1 hour)
         └─── Stateless: No server-side session storage needed
```

## Security Layers

```
                        Request Flow with Security

        Browser                                    Server
          │                                          │
          │  1. HTTP Request                         │
          ├─────────────────────────────────────────>│
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 1: HTTPS    │
          │                      │ - TLS encryption  │
          │                      │ - Certificate     │
          │                      └───────────────────┤
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 2: Cookie   │
          │                      │ - HttpOnly flag   │
          │                      │ - Secure flag     │
          │                      │ - SameSite=Strict │
          │                      └───────────────────┤
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 3: JWT      │
          │                      │ - Signature check │
          │                      │ - Expiration check│
          │                      │ - Claims validate │
          │                      └───────────────────┤
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 4: RBAC     │
          │                      │ - Role check      │
          │                      │ - Permission check│
          │                      └───────────────────┤
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 5: Rate Limit
          │                      │ - Login attempts  │
          │                      │ - API throttling  │
          │                      └───────────────────┤
          │                                          │
          │                      ┌───────────────────┤
          │                      │ Layer 6: Audit    │
          │                      │ - Log all actions │
          │                      │ - Track anomalies │
          │                      └───────────────────┤
          │                                          │
          │  2. Response (if all layers pass)        │
          │<─────────────────────────────────────────┤
          │                                          │
```

## Concurrent Access Protection

```
Multiple Processes Accessing users.json

   Process A                 File System              Process B
      │                          │                       │
      │ 1. Lock file             │                       │
      ├─────────────────────────>│                       │
      │                          │                       │
      │ 2. Lock acquired         │                       │
      │ (.users.lock created)    │                       │
      │<─────────────────────────┤                       │
      │                          │                       │
      │ 3. Read users.json       │                       │
      ├─────────────────────────>│                       │
      │                          │    4. Try lock file   │
      │                          │<──────────────────────┤
      │                          │                       │
      │                          │    5. Lock BLOCKED    │
      │                          │    (wait)             │
      │                          ├──────────────────────>│
      │                          │                       │
      │ 6. Modify data           │                       │
      │                          │                       │
      │ 7. Write users.json.tmp  │                       │
      ├─────────────────────────>│                       │
      │                          │                       │
      │ 8. Rename tmp → json     │                       │
      ├─────────────────────────>│                       │
      │                          │                       │
      │ 9. Unlock file           │                       │
      │ (remove .users.lock)     │                       │
      ├─────────────────────────>│                       │
      │                          │                       │
      │                          │    10. Lock acquired  │
      │                          │<──────────────────────┤
      │                          │                       │
      │                          │    11. Read users.json│
      │                          │<──────────────────────┤
      │                          │                       │
      │                          │    12. Process safely │
      │                          │                       │
```

This ensures **no race conditions** even with parallel user operations!
