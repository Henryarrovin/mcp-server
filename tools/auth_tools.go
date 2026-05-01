package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Henryarrovin/mcp-server/mcp"
)

func RegisterAuthTools(s *mcp.Server, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	do := func(method, url, token, body string) (string, error) {
		var r io.Reader
		if body != "" {
			r = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, url, r)
		if err != nil {
			return "", err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return string(data), nil
	}

	// Login
	s.AddTool(
		mcp.NewTool("auth_login",
			"Login with email and password — returns access and refresh tokens",
			map[string]mcp.Property{
				"email":    mcp.Str("User email address"),
				"password": mcp.Str("User password"),
			},
			[]string{"email", "password"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			email := mcp.GetString(args, "email", "")
			pass := mcp.GetString(args, "password", "")
			body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, pass)
			result, err := do(http.MethodPost, baseURL+"/api/v1/auth/login", "", body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Register
	s.AddTool(
		mcp.NewTool("auth_register",
			"Register a new user account",
			map[string]mcp.Property{
				"email":    mcp.Str("User email"),
				"password": mcp.Str("User password"),
				"name":     mcp.Str("User full name"),
				"role":     mcp.Str("Role: user or admin (default: user)"),
			},
			[]string{"email", "password", "name"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			email := mcp.GetString(args, "email", "")
			pass := mcp.GetString(args, "password", "")
			name := mcp.GetString(args, "name", "")
			role := mcp.GetString(args, "role", "user")
			body := fmt.Sprintf(`{"email":%q,"password":%q,"name":%q,"role":%q}`, email, pass, name, role)
			result, err := do(http.MethodPost, baseURL+"/api/v1/auth/register", "", body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Validate Token
	s.AddTool(
		mcp.NewTool("auth_validate_token",
			"Validate a JWT token and return the user info",
			map[string]mcp.Property{
				"token": mcp.Str("JWT access token"),
			},
			[]string{"token"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			result, err := do(http.MethodGet, baseURL+"/api/v1/auth/me", token, "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Refresh Token
	s.AddTool(
		mcp.NewTool("auth_refresh_token",
			"Refresh an access token using a refresh token",
			map[string]mcp.Property{
				"refresh_token": mcp.Str("JWT refresh token"),
			},
			[]string{"refresh_token"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			rt := mcp.GetString(args, "refresh_token", "")
			body := fmt.Sprintf(`{"refresh_token":%q}`, rt)
			result, err := do(http.MethodPost, baseURL+"/api/v1/auth/refresh", "", body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Logout
	s.AddTool(
		mcp.NewTool("auth_logout",
			"Logout and revoke the current access token",
			map[string]mcp.Property{
				"token": mcp.Str("JWT access token to revoke"),
			},
			[]string{"token"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			result, err := do(http.MethodPost, baseURL+"/api/v1/auth/logout", token, "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Health Check
	s.AddTool(
		mcp.NewTool("auth_health",
			"Check if auth-service is healthy and reachable",
			map[string]mcp.Property{},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := do(http.MethodGet, baseURL+"/api/v1/auth/health", "", "")
			if err != nil {
				return mcp.ErrorResult("auth-service unreachable: " + err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)
}
