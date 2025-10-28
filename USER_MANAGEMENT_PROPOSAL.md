# User Management Proposal - Filesystem-Based Authentication

## Executive Summary

This proposal outlines a secure, filesystem-based user management system for the GraphDB Service Web UI. Since no database is available, we'll use encrypted JSON files for user storage with bcrypt password hashing, JWT-based session management, and configurable authentication modes.

## Requirements Analysis

### Current State
- ✅ API endpoints protected by API key (`x-api-key` header)
- ❌ Web UI is publicly accessible (no authentication)
- ❌ No user management system
- ❌ No session management for UI users
- ❌ No audit logging for user actions

### Constraints
- **No database available** - Must use filesystem storage
- **Production-ready** - Must be secure and performant
- **Simple deployment** - Minimal configuration required
- **Backward compatible** - API key authentication must continue working
- **Multi-user support** - Multiple concurrent authenticated users

### Goals
- ✅ Secure Web UI access with username/password authentication
- ✅ User registration and management (admin-controlled)
- ✅ Session management with automatic expiration
- ✅ Password security (bcrypt hashing, salting)
- ✅ Audit logging (who did what, when)
- ✅ Role-based access control (admin, user)
- ✅ Optional: Self-registration with approval workflow

---

## Architecture Overview

### 1. Authentication Modes

Support **three authentication modes** (configurable via environment variable):

#### Mode 1: No Authentication (Default - Current Behavior)
```bash
AUTH_MODE=none  # Default, no login required
```
- Web UI accessible to everyone
- API still requires `x-api-key`
- Use for development or trusted internal networks

#### Mode 2: Simple Authentication
```bash
AUTH_MODE=simple
```
- All users have same permissions
- Login required for Web UI
- No role differentiation
- Suitable for small teams

#### Mode 3: Role-Based Authentication (Recommended)
```bash
AUTH_MODE=rbac
```
- Admin users: Full access + user management
- Regular users: Can only execute tasks
- Login required for Web UI
- Enterprise-ready with audit logging

### 2. Filesystem Storage Structure

```
/data/
├── users/
│   ├── users.json              # User database (encrypted)
│   ├── users.json.backup       # Automatic backup
│   └── .users.lock             # File lock for concurrent access
├── sessions/
│   ├── sessions.json           # Active sessions
│   └── cleanup.log             # Session cleanup log
└── audit/
    ├── 2025-10/
    │   ├── 2025-10-28.log     # Daily audit log
    │   └── 2025-10-29.log
    └── retention.json          # Retention policy config
```

**Configuration via environment variables:**
```bash
DATA_DIR=/data                   # Default: ./data
AUTH_MODE=rbac                   # Options: none, simple, rbac
SESSION_TIMEOUT=3600             # Default: 1 hour (in seconds)
PASSWORD_MIN_LENGTH=8            # Default: 8
REQUIRE_STRONG_PASSWORD=true    # Default: true
AUDIT_RETENTION_DAYS=90         # Default: 90 days
```

---

## Data Models

### User Model

```go
type User struct {
    ID             string    `json:"id"`              // UUID
    Username       string    `json:"username"`        // Unique, 3-50 chars
    Email          string    `json:"email"`           // Optional
    PasswordHash   string    `json:"password_hash"`   // bcrypt hash
    Role           string    `json:"role"`            // "admin" or "user"
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
    LastLoginAt    *time.Time `json:"last_login_at"`
    FailedLogins   int       `json:"failed_logins"`   // Rate limiting
    Locked         bool      `json:"locked"`          // Account locked
    MustChangePassword bool  `json:"must_change_password"`
}

type UserDatabase struct {
    Version    string           `json:"version"`      // Schema version
    Users      map[string]User  `json:"users"`        // Key: username
    UpdatedAt  time.Time        `json:"updated_at"`
}
```

**File**: `data/users/users.json`
```json
{
  "version": "1.0.0",
  "users": {
    "admin": {
      "id": "a3bb189e-2b8f-4d91-9c4a-f8e7d6c5b4a3",
      "username": "admin",
      "email": "admin@example.com",
      "password_hash": "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
      "role": "admin",
      "created_at": "2025-10-28T10:00:00Z",
      "updated_at": "2025-10-28T10:00:00Z",
      "last_login_at": "2025-10-28T12:30:00Z",
      "failed_logins": 0,
      "locked": false,
      "must_change_password": false
    }
  },
  "updated_at": "2025-10-28T10:00:00Z"
}
```

### Session Model

```go
type Session struct {
    ID          string    `json:"id"`           // UUID
    UserID      string    `json:"user_id"`
    Username    string    `json:"username"`
    Role        string    `json:"role"`
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at"`
    LastActivity time.Time `json:"last_activity"`
    IPAddress   string    `json:"ip_address"`
    UserAgent   string    `json:"user_agent"`
}

type SessionStore struct {
    Sessions  map[string]Session `json:"sessions"`  // Key: session ID
    UpdatedAt time.Time          `json:"updated_at"`
}
```

**Storage Options:**

#### Option A: JWT Tokens (Recommended)
- Store in HTTP-only secure cookies
- Signed with secret key
- Stateless (no server-side storage needed)
- Auto-expire based on token expiration

#### Option B: Session File + Cookie
- Cookie contains session ID only
- Session data stored in `sessions.json`
- Requires cleanup of expired sessions
- More server-side control

**Recommendation**: Use JWT with optional session tracking file for audit purposes.

### Audit Log Model

```go
type AuditEntry struct {
    ID        string                 `json:"id"`         // UUID
    Timestamp time.Time              `json:"timestamp"`
    UserID    string                 `json:"user_id"`
    Username  string                 `json:"username"`
    Action    string                 `json:"action"`     // "login", "task_execute", etc.
    Resource  string                 `json:"resource"`   // What was affected
    Details   map[string]interface{} `json:"details"`    // Additional context
    IPAddress string                 `json:"ip_address"`
    Success   bool                   `json:"success"`
    Error     string                 `json:"error,omitempty"`
}
```

**File**: `data/audit/2025-10/2025-10-28.log` (JSONL format)
```jsonl
{"id":"uuid1","timestamp":"2025-10-28T10:00:00Z","user_id":"uuid","username":"admin","action":"login","resource":"auth","details":{},"ip_address":"192.168.1.100","success":true}
{"id":"uuid2","timestamp":"2025-10-28T10:05:00Z","user_id":"uuid","username":"admin","action":"task_execute","resource":"repo-migration","details":{"src_repo":"prod-repo","tgt_repo":"backup-repo"},"ip_address":"192.168.1.100","success":true}
```

---

## Security Implementation

### 1. Password Security

```go
import "golang.org/x/crypto/bcrypt"

// Hash password with bcrypt (cost factor: 10)
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
    return string(bytes), err
}

// Verify password
func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// Validate password strength
func ValidatePassword(password string) error {
    if len(password) < 8 {
        return errors.New("password must be at least 8 characters")
    }

    var (
        hasUpper   = regexp.MustCompile(`[A-Z]`).MatchString(password)
        hasLower   = regexp.MustCompile(`[a-z]`).MatchString(password)
        hasNumber  = regexp.MustCompile(`[0-9]`).MatchString(password)
        hasSpecial = regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)
    )

    if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
        return errors.New("password must contain uppercase, lowercase, number, and special character")
    }

    return nil
}
```

### 2. JWT Token Implementation

```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

// Generate JWT token
func GenerateToken(user User, secret string, expirationHours int) (string, error) {
    claims := Claims{
        UserID:   user.ID,
        Username: user.Username,
        Role:     user.Role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(expirationHours))),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "graphservice",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

// Validate JWT token
func ValidateToken(tokenString string, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, errors.New("invalid token")
}
```

### 3. Rate Limiting (Brute Force Protection)

```go
type LoginAttempt struct {
    Username  string
    Timestamp time.Time
}

var loginAttempts = make(map[string][]LoginAttempt)
var attemptsMutex sync.RWMutex

// Check if user is rate limited
func IsRateLimited(username string) bool {
    attemptsMutex.RLock()
    defer attemptsMutex.RUnlock()

    attempts, exists := loginAttempts[username]
    if !exists {
        return false
    }

    // Count attempts in last 15 minutes
    recentAttempts := 0
    cutoff := time.Now().Add(-15 * time.Minute)

    for _, attempt := range attempts {
        if attempt.Timestamp.After(cutoff) {
            recentAttempts++
        }
    }

    return recentAttempts >= 5 // Max 5 attempts per 15 minutes
}

// Record login attempt
func RecordLoginAttempt(username string) {
    attemptsMutex.Lock()
    defer attemptsMutex.Unlock()

    loginAttempts[username] = append(loginAttempts[username], LoginAttempt{
        Username:  username,
        Timestamp: time.Now(),
    })

    // Cleanup old attempts
    cleanupLoginAttempts(username)
}
```

### 4. File-Based Storage Security

```go
import "github.com/gofrs/flock"

// Thread-safe file operations with file locking
func SaveUserDatabase(db *UserDatabase, filepath string) error {
    // Acquire file lock
    lock := flock.New(filepath + ".lock")
    locked, err := lock.TryLock()
    if err != nil {
        return err
    }
    if !locked {
        return errors.New("unable to acquire lock")
    }
    defer lock.Unlock()

    // Create backup of existing file
    if _, err := os.Stat(filepath); err == nil {
        backupPath := filepath + ".backup"
        if err := os.Rename(filepath, backupPath); err != nil {
            return fmt.Errorf("failed to create backup: %w", err)
        }
    }

    // Marshal to JSON with indentation
    data, err := json.MarshalIndent(db, "", "  ")
    if err != nil {
        return err
    }

    // Write atomically (write to temp, then rename)
    tempFile := filepath + ".tmp"
    if err := os.WriteFile(tempFile, data, 0600); err != nil {
        return err
    }

    if err := os.Rename(tempFile, filepath); err != nil {
        return err
    }

    return nil
}
```

---

## Authentication Flow

### 1. Login Flow

```
┌─────────┐                                           ┌─────────────┐
│ Browser │                                           │   Server    │
└────┬────┘                                           └──────┬──────┘
     │                                                        │
     │  GET /login                                           │
     ├──────────────────────────────────────────────────────>│
     │                                                        │
     │  <-- Login Form HTML                                  │
     │<──────────────────────────────────────────────────────┤
     │                                                        │
     │  POST /auth/login                                     │
     │  {username, password}                                 │
     ├──────────────────────────────────────────────────────>│
     │                                                        │
     │                                   [Validate credentials]
     │                                   [Check rate limiting]
     │                                   [Generate JWT token]
     │                                   [Update last_login_at]
     │                                   [Log audit entry]
     │                                                        │
     │  Set-Cookie: session=<JWT>                            │
     │  Redirect to /                                        │
     │<──────────────────────────────────────────────────────┤
     │                                                        │
     │  GET /                                                │
     │  Cookie: session=<JWT>                                │
     ├──────────────────────────────────────────────────────>│
     │                                                        │
     │                                        [Validate token]
     │                                        [Extract claims]
     │                                                        │
     │  <-- Dashboard HTML (authenticated)                   │
     │<──────────────────────────────────────────────────────┤
     │                                                        │
```

### 2. Middleware Authentication Check

```go
func AuthMiddleware(authMode string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Skip authentication if mode is "none"
            if authMode == "none" {
                return next(c)
            }

            // Check for JWT token in cookie
            cookie, err := c.Cookie("session")
            if err != nil {
                return c.Redirect(http.StatusFound, "/login")
            }

            // Validate token
            claims, err := ValidateToken(cookie.Value, getJWTSecret())
            if err != nil {
                return c.Redirect(http.StatusFound, "/login")
            }

            // Store user info in context
            c.Set("user_id", claims.UserID)
            c.Set("username", claims.Username)
            c.Set("role", claims.Role)

            return next(c)
        }
    }
}

// Apply to UI routes
func RegisterUIRoutes(e *echo.Echo, authMode string) {
    // Public routes (no auth required)
    e.GET("/login", loginPageHandler)
    e.POST("/auth/login", loginHandler)
    e.GET("/health", healthHandler)

    // Protected routes (auth required)
    ui := e.Group("", AuthMiddleware(authMode))
    ui.GET("/", uiIndexHandler)
    ui.GET("/ui", uiIndexHandler)
    ui.POST("/ui/execute", uiExecuteHandler)
    ui.GET("/ui/stream/:sessionID", uiStreamHandler)
    ui.GET("/logout", logoutHandler)

    // Admin-only routes (if RBAC mode)
    if authMode == "rbac" {
        admin := e.Group("/admin", AuthMiddleware(authMode), AdminOnlyMiddleware())
        admin.GET("/users", listUsersHandler)
        admin.POST("/users", createUserHandler)
        admin.PUT("/users/:username", updateUserHandler)
        admin.DELETE("/users/:username", deleteUserHandler)
    }
}
```

---

## User Interface Components

### 1. Login Page

```html
<!DOCTYPE html>
<html>
<head>
    <title>Login - GraphDB Service</title>
    <style>
        /* Pantopix corporate design styles */
        .login-container {
            max-width: 400px;
            margin: 100px auto;
            padding: 2rem;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .login-form input {
            width: 100%;
            padding: 0.75rem;
            margin-bottom: 1rem;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        .login-form button {
            width: 100%;
            padding: 0.75rem;
            background: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .error-message {
            color: #dc3545;
            margin-bottom: 1rem;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <h2>GraphDB Service</h2>
        <p>Please login to continue</p>

        {{if .Error}}
        <div class="error-message">{{.Error}}</div>
        {{end}}

        <form class="login-form" action="/auth/login" method="POST">
            <input type="text" name="username" placeholder="Username" required>
            <input type="password" name="password" placeholder="Password" required>
            <button type="submit">Login</button>
        </form>
    </div>
</body>
</html>
```

### 2. User Management UI (Admin Only)

```html
<div class="admin-panel">
    <h2>User Management</h2>

    <button onclick="showCreateUserModal()">+ Create User</button>

    <table>
        <thead>
            <tr>
                <th>Username</th>
                <th>Email</th>
                <th>Role</th>
                <th>Last Login</th>
                <th>Status</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody id="users-table">
            <!-- Populated via HTMX -->
        </tbody>
    </table>
</div>

<div id="create-user-modal" class="modal" style="display: none;">
    <form hx-post="/admin/users" hx-target="#users-table">
        <input type="text" name="username" placeholder="Username" required>
        <input type="email" name="email" placeholder="Email">
        <input type="password" name="password" placeholder="Password" required>
        <select name="role">
            <option value="user">User</option>
            <option value="admin">Admin</option>
        </select>
        <button type="submit">Create User</button>
        <button type="button" onclick="hideCreateUserModal()">Cancel</button>
    </form>
</div>
```

### 3. User Info Display (Navbar)

```html
<nav>
    <div class="user-info">
        <span>Logged in as: <strong>{{.Username}}</strong></span>
        {{if eq .Role "admin"}}
        <span class="badge">Admin</span>
        <a href="/admin/users">Manage Users</a>
        {{end}}
        <a href="/logout">Logout</a>
    </div>
</nav>
```

---

## Implementation Phases

### Phase 1: Core Authentication (Week 1)
**Goal**: Basic login/logout functionality

- [ ] Add user model and filesystem storage
- [ ] Implement bcrypt password hashing
- [ ] Create JWT token generation and validation
- [ ] Build login page UI (templ template)
- [ ] Add authentication middleware
- [ ] Implement logout handler
- [ ] Add session cookie management
- [ ] Environment variable configuration

**Deliverables**:
- Users can login with username/password
- Sessions expire after configured timeout
- Logout clears session
- AUTH_MODE environment variable controls behavior

### Phase 2: User Management (Week 2)
**Goal**: Admin can manage users

- [ ] Create initial admin user on first startup
- [ ] Build admin UI for user management
- [ ] Implement CRUD endpoints for users
- [ ] Add role-based access control
- [ ] Password change functionality
- [ ] Account locking mechanism
- [ ] Rate limiting for login attempts

**Deliverables**:
- Admin can create/edit/delete users
- Admin can assign roles
- Admin can lock/unlock accounts
- Users can change their own password

### Phase 3: Audit Logging (Week 3)
**Goal**: Track all user actions

- [ ] Implement audit log model
- [ ] Add logging to all authenticated actions
- [ ] Create audit log viewer UI (admin only)
- [ ] Add log rotation and retention
- [ ] Export audit logs functionality
- [ ] Search/filter audit logs

**Deliverables**:
- All actions logged with user, timestamp, details
- Logs rotated daily
- Logs retained for configured period
- Admin can view and export logs

### Phase 4: Advanced Features (Future)
**Optional enhancements**:

- [ ] Self-registration with email verification
- [ ] Password reset via email
- [ ] Two-factor authentication (TOTP)
- [ ] LDAP/Active Directory integration
- [ ] OAuth2/OIDC integration
- [ ] API token generation for users
- [ ] IP whitelist/blacklist
- [ ] Detailed permission matrix

---

## Migration Strategy

### Step 1: Add Feature Flag
```go
// Maintain backward compatibility
authEnabled := os.Getenv("AUTH_MODE") != "none"
if authEnabled {
    // Apply authentication middleware
} else {
    // Keep current behavior (no auth)
}
```

### Step 2: Initial Admin User
On first startup with `AUTH_MODE != none`:
```bash
# Auto-generate admin user with random password
Starting GraphDB Service with authentication enabled...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  INITIAL ADMIN CREDENTIALS
  Username: admin
  Password: Xk9#mP2$vL8!nQ4@

  ⚠️  IMPORTANT: Change this password immediately!
  ⚠️  These credentials will not be shown again.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Step 3: Environment Variable Configuration
```bash
# .env file
AUTH_MODE=rbac
JWT_SECRET=<random-64-char-string>
SESSION_TIMEOUT=3600
DATA_DIR=/data
PASSWORD_MIN_LENGTH=8
REQUIRE_STRONG_PASSWORD=true
MAX_LOGIN_ATTEMPTS=5
LOGIN_ATTEMPT_WINDOW=900
AUDIT_RETENTION_DAYS=90
```

---

## Security Considerations

### 1. JWT Secret Management
```bash
# Generate strong secret
openssl rand -base64 64 > jwt-secret.txt
export JWT_SECRET=$(cat jwt-secret.txt)
```

**Never**:
- Commit JWT secret to version control
- Use default/example secrets in production
- Share secrets via insecure channels

### 2. File Permissions
```bash
# Restrict access to user data
chmod 600 data/users/users.json
chmod 700 data/users/
chmod 700 data/sessions/
chmod 755 data/audit/  # Read access for log analysis tools
```

### 3. HTTPS Requirement
**Production**: Always use HTTPS for:
- Secure cookie transmission (Secure flag)
- Prevent credential interception
- TLS 1.2+ only

### 4. Cookie Security Flags
```go
cookie := &http.Cookie{
    Name:     "session",
    Value:    token,
    Path:     "/",
    HttpOnly: true,  // Prevent XSS attacks
    Secure:   true,  // HTTPS only
    SameSite: http.SameSiteStrictMode,  // CSRF protection
    MaxAge:   sessionTimeout,
}
```

### 5. Input Validation
- Username: 3-50 chars, alphanumeric + underscore/hyphen
- Password: Min 8 chars (configurable), complexity rules
- Email: Valid email format (if required)
- Sanitize all user inputs to prevent injection

---

## Alternative Approaches Considered

### Alternative 1: SQLite Database
**Pros**:
- Structured queries
- ACID transactions
- Better for complex queries

**Cons**:
- Adds dependency
- Overkill for simple user management
- Requires migration scripts

**Decision**: Rejected - JSON files sufficient for small user base

### Alternative 2: External Authentication Service
**Pros**:
- Offload authentication complexity
- Enterprise features (SSO, MFA)
- No user management code

**Cons**:
- External dependency
- Network latency
- Additional cost
- Requires internet connection

**Decision**: Rejected for v1, consider for future

### Alternative 3: Basic Auth (HTTP)
**Pros**:
- Simple to implement
- No session management needed
- Stateless

**Cons**:
- Credentials sent with every request
- No proper logout
- Poor UX (browser popup)
- No role management

**Decision**: Rejected - poor UX and security

---

## Testing Strategy

### 1. Unit Tests
```go
func TestHashPassword(t *testing.T) { /* ... */ }
func TestCheckPasswordHash(t *testing.T) { /* ... */ }
func TestValidatePassword(t *testing.T) { /* ... */ }
func TestGenerateToken(t *testing.T) { /* ... */ }
func TestValidateToken(t *testing.T) { /* ... */ }
func TestSaveUserDatabase(t *testing.T) { /* ... */ }
func TestLoadUserDatabase(t *testing.T) { /* ... */ }
```

### 2. Integration Tests
```go
func TestLoginFlow(t *testing.T) {
    // Test successful login
    // Test invalid credentials
    // Test rate limiting
    // Test session expiration
}

func TestUserManagement(t *testing.T) {
    // Test user creation
    // Test user update
    // Test user deletion
    // Test role enforcement
}
```

### 3. Security Tests
- Password brute force protection
- JWT token tampering detection
- Session hijacking prevention
- XSS/CSRF protection
- SQL injection (in JSON queries)
- Path traversal in file operations

---

## Performance Considerations

### File I/O Optimization
- **Read-heavy workload**: Cache user database in memory
- **Write operations**: Use file locking + atomic writes
- **Session validation**: JWT (no file I/O per request)

### Scalability Limits
- **Users**: Up to 10,000 users (100KB JSON file)
- **Sessions**: Up to 1,000 concurrent sessions
- **Audit logs**: Daily rotation, compression

### Horizontal Scaling
For multiple instances:
- **Users file**: Shared filesystem (NFS/EFS) with file locking
- **Sessions**: JWT (stateless) or shared Redis
- **Audit logs**: Each instance writes to own file, merge later

---

## Recommendation

**Recommended Implementation**:

1. **Start with Phase 1** (Core Authentication)
   - AUTH_MODE=none by default (backward compatible)
   - JWT-based sessions (stateless, performant)
   - bcrypt password hashing (industry standard)
   - File-based user storage with locking

2. **Authentication Mode**: RBAC (Role-Based Access Control)
   - Admin role: Full access + user management
   - User role: Execute tasks only
   - Simple to implement, covers most use cases

3. **Storage**: JSON files with file locking
   - Simple, no external dependencies
   - Sufficient for <10k users
   - Easy backup and migration

4. **Security**: Production-grade from day 1
   - HTTPS required
   - Secure cookies (HttpOnly, Secure, SameSite)
   - Rate limiting
   - Strong password requirements

5. **Timeline**: 3 weeks for full implementation
   - Week 1: Core auth (MVP - usable)
   - Week 2: User management (complete)
   - Week 3: Audit logging (production-ready)

**Next Steps**:
1. Review and approve this proposal
2. Set environment variables (AUTH_MODE, JWT_SECRET, etc.)
3. Implement Phase 1 (Core Authentication)
4. Test with internal users
5. Roll out Phases 2 & 3

---

## Questions for Discussion

1. **User capacity**: How many users do you expect? (affects storage choice)
2. **Self-registration**: Should users be able to register themselves? (requires approval workflow)
3. **Password policy**: Enforce complexity rules? Password expiration?
4. **Integration**: Need LDAP/AD integration for enterprise SSO?
5. **Audit requirements**: Compliance requirements (GDPR, SOC2, etc.)?
6. **Deployment**: Single instance or multiple instances? (affects session storage)
7. **Email**: Do you have SMTP server for password reset emails?

Please provide feedback on:
- Overall approach
- Security requirements
- Timeline and priorities
- Any specific compliance needs
