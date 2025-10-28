package cmd

import (
	"net/http"
	"time"

	"evalgo.org/graphservice/auth"
	"evalgo.org/graphservice/web/templates"
	"github.com/labstack/echo/v4"
)

// UserResponse represents the user data returned by the API (without password hash)
type UserResponse struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	Role               string     `json:"role"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	FailedLogins       int        `json:"failed_logins"`
	Locked             bool       `json:"locked"`
	MustChangePassword bool       `json:"must_change_password"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	MustChangePassword bool   `json:"must_change_password"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email              *string `json:"email,omitempty"`
	Password           *string `json:"password,omitempty"`
	Role               *string `json:"role,omitempty"`
	Locked             *bool   `json:"locked,omitempty"`
	MustChangePassword *bool   `json:"must_change_password,omitempty"`
	ResetFailedLogins  bool    `json:"reset_failed_logins,omitempty"`
}

// userToResponse converts a User to UserResponse (removes sensitive data)
func userToResponse(user *auth.User) UserResponse {
	return UserResponse{
		ID:                 user.ID,
		Username:           user.Username,
		Email:              user.Email,
		Role:               user.Role,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
		LastLoginAt:        user.LastLoginAt,
		FailedLogins:       user.FailedLogins,
		Locked:             user.Locked,
		MustChangePassword: user.MustChangePassword,
	}
}

// usersPageHandler serves the user management page (admin only)
func usersPageHandler(c echo.Context) error {
	// Get user from context
	var user *auth.User
	if val := c.Get("user"); val != nil {
		if u, ok := val.(*auth.User); ok {
			user = u
		}
	}

	component := templates.Users(user)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// listUsersHandler returns all users for HTMX (admin only)
func listUsersHandler(c echo.Context) error {
	users, err := userStore.ListUsers()
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "<p>Failed to load users</p>")
	}

	// Convert to map for template
	userMaps := make([]map[string]interface{}, len(users))
	for i, user := range users {
		userMaps[i] = map[string]interface{}{
			"id":                   user.ID,
			"username":             user.Username,
			"email":                user.Email,
			"role":                 user.Role,
			"locked":               user.Locked,
			"failed_logins":        user.FailedLogins,
			"must_change_password": user.MustChangePassword,
			"last_login_at":        user.LastLoginAt,
		}
	}

	component := templates.UsersList(userMaps)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// listUsersAPIHandler returns all users as JSON (admin only)
func listUsersAPIHandler(c echo.Context) error {
	users, err := userStore.ListUsers()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list users",
		})
	}

	// Convert to response format
	responses := make([]UserResponse, len(users))
	for i, user := range users {
		responses[i] = userToResponse(&user)
	}

	return c.JSON(http.StatusOK, responses)
}

// getUserHandler returns a single user by username (admin only)
func getUserHandler(c echo.Context) error {
	username := c.Param("username")

	user, err := userStore.GetUser(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	return c.JSON(http.StatusOK, userToResponse(user))
}

// createUserHandler creates a new user (admin only)
func createUserHandler(c echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Validate username
	if err := auth.ValidateUsername(req.Username); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Validate password
	if err := auth.ValidatePassword(req.Password, 8, true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Validate role
	if req.Role != auth.RoleAdmin && req.Role != auth.RoleUser {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid role. Must be 'admin' or 'user'",
		})
	}

	// Create user
	user, err := userStore.CreateUser(req.Username, req.Password, req.Email, req.Role)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": err.Error(),
		})
	}

	// Set must change password if requested
	if req.MustChangePassword {
		updates := map[string]interface{}{
			"must_change_password": true,
		}
		_ = userStore.UpdateUser(user.Username, updates)
		user.MustChangePassword = true
	}

	// Log audit entry
	currentUser := c.Get("username").(string)
	logAudit(c, c.Get("user_id").(string), currentUser, "create_user", "user:"+user.Username, true, "", map[string]interface{}{
		"new_username": user.Username,
		"role":         user.Role,
	})

	return c.JSON(http.StatusCreated, userToResponse(user))
}

// updateUserHandler updates an existing user (admin only)
func updateUserHandler(c echo.Context) error {
	username := c.Param("username")

	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Check if user exists
	if _, err := userStore.GetUser(username); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	// Build updates map
	updates := make(map[string]interface{})

	if req.Email != nil {
		updates["email"] = *req.Email
	}

	if req.Password != nil {
		// Validate new password
		if err := auth.ValidatePassword(*req.Password, 8, true); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		updates["password"] = *req.Password
	}

	if req.Role != nil {
		if *req.Role != auth.RoleAdmin && *req.Role != auth.RoleUser {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid role. Must be 'admin' or 'user'",
			})
		}
		updates["role"] = *req.Role
	}

	if req.Locked != nil {
		updates["locked"] = *req.Locked
	}

	if req.MustChangePassword != nil {
		updates["must_change_password"] = *req.MustChangePassword
	}

	if req.ResetFailedLogins {
		updates["failed_logins"] = 0
	}

	// Apply updates
	if err := userStore.UpdateUser(username, updates); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Get updated user
	user, _ := userStore.GetUser(username)

	// Log audit entry
	currentUser := c.Get("username").(string)
	logAudit(c, c.Get("user_id").(string), currentUser, "update_user", "user:"+username, true, "", map[string]interface{}{
		"target_username": username,
		"updates":         updates,
	})

	return c.JSON(http.StatusOK, userToResponse(user))
}

// deleteUserHandler deletes a user (admin only)
func deleteUserHandler(c echo.Context) error {
	username := c.Param("username")

	// Prevent deleting yourself
	currentUsername := c.Get("username").(string)
	if username == currentUsername {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Cannot delete your own account",
		})
	}

	// Delete user
	if err := userStore.DeleteUser(username); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	// Log audit entry
	logAudit(c, c.Get("user_id").(string), currentUsername, "delete_user", "user:"+username, true, "", map[string]interface{}{
		"deleted_username": username,
	})

	return c.JSON(http.StatusOK, map[string]string{
		"message": "User deleted successfully",
	})
}

// changePasswordPageHandler serves the password change page
func changePasswordPageHandler(c echo.Context) error {
	// Get user from context
	var user *auth.User
	if val := c.Get("user"); val != nil {
		if u, ok := val.(*auth.User); ok {
			user = u
		}
	}

	component := templates.ChangePassword(user)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// changePasswordHandler allows users to change their own password
func changePasswordHandler(c echo.Context) error {
	username := c.Get("username").(string)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	// Get user
	user, err := userStore.GetUser(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	// Verify current password
	if !auth.CheckPasswordHash(req.CurrentPassword, user.PasswordHash) {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Current password is incorrect",
		})
	}

	// Validate new password
	if err := auth.ValidatePassword(req.NewPassword, 8, true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Update password
	updates := map[string]interface{}{
		"password":             req.NewPassword,
		"must_change_password": false,
	}

	if err := userStore.UpdateUser(username, updates); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update password",
		})
	}

	// Log audit entry
	logAudit(c, user.ID, username, "change_password", "user:"+username, true, "", nil)

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Password changed successfully",
	})
}

// getCurrentUserHandler returns the current user's info
func getCurrentUserHandler(c echo.Context) error {
	username := c.Get("username").(string)

	user, err := userStore.GetUser(username)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
	}

	return c.JSON(http.StatusOK, userToResponse(user))
}
