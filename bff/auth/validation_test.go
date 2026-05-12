package auth

import (
	"bff/setup"
	"testing"
)

func Test_isAllowedRedirectPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Pattern not configured - allow all
		{
			name:    "No pattern configured - allow root",
			path:    "/",
			pattern: "",
			want:    true,
		},
		{
			name:    "No pattern configured - allow any path",
			path:    "/dashboard",
			pattern: "",
			want:    true,
		},
		{
			name:    "No pattern configured - allow nested path",
			path:    "/user/123/profile",
			pattern: "",
			want:    true,
		},

		// Exact match patterns
		{
			name:    "Exact match - root allowed",
			path:    "/",
			pattern: "^/$",
			want:    true,
		},
		{
			name:    "Exact match - dashboard not allowed",
			path:    "/dashboard",
			pattern: "^/$",
			want:    false,
		},
		{
			name:    "Exact match - specific paths",
			path:    "/dashboard",
			pattern: "^/(dashboard|profile|settings)$",
			want:    true,
		},
		{
			name:    "Exact match - profile allowed",
			path:    "/profile",
			pattern: "^/(dashboard|profile|settings)$",
			want:    true,
		},
		{
			name:    "Exact match - settings allowed",
			path:    "/settings",
			pattern: "^/(dashboard|profile|settings)$",
			want:    true,
		},
		{
			name:    "Exact match - admin not allowed",
			path:    "/admin",
			pattern: "^/(dashboard|profile|settings)$",
			want:    false,
		},

		// Subpath patterns
		{
			name:    "Subpath - dashboard with nested path allowed",
			path:    "/dashboard/users",
			pattern: "^/(dashboard|profile|settings)(/.*)?$",
			want:    true,
		},
		{
			name:    "Subpath - dashboard root allowed",
			path:    "/dashboard",
			pattern: "^/(dashboard|profile|settings)(/.*)?$",
			want:    true,
		},
		{
			name:    "Subpath - deeply nested path allowed",
			path:    "/dashboard/users/123/edit",
			pattern: "^/(dashboard|profile|settings)(/.*)?$",
			want:    true,
		},
		{
			name:    "Subpath - admin not allowed",
			path:    "/admin/users",
			pattern: "^/(dashboard|profile|settings)(/.*)?$",
			want:    false,
		},

		// Prefix match patterns
		{
			name:    "Prefix match - app with nested path",
			path:    "/app/dashboard",
			pattern: "^/app/.*$",
			want:    true,
		},
		{
			name:    "Prefix match - app root not allowed (requires slash after)",
			path:    "/app",
			pattern: "^/app/.*$",
			want:    false,
		},
		{
			name:    "Prefix match - deeply nested path",
			path:    "/app/users/123/profile",
			pattern: "^/app/.*$",
			want:    true,
		},
		{
			name:    "Prefix match - api not allowed",
			path:    "/api/users",
			pattern: "^/app/.*$",
			want:    false,
		},

		// Complex patterns
		{
			name:    "Complex - public allowed",
			path:    "/public",
			pattern: `^/(public|user/[^/]+/dashboard)$`,
			want:    true,
		},
		{
			name:    "Complex - user dashboard allowed",
			path:    "/user/alice/dashboard",
			pattern: `^/(public|user/[^/]+/dashboard)$`,
			want:    true,
		},
		{
			name:    "Complex - user profile not allowed",
			path:    "/user/alice/profile",
			pattern: `^/(public|user/[^/]+/dashboard)$`,
			want:    false,
		},
		{
			name:    "Complex - nested user path not allowed",
			path:    "/user/alice/dashboard/settings",
			pattern: `^/(public|user/[^/]+/dashboard)$`,
			want:    false,
		},

		// Query parameters in path (should work with path only)
		{
			name:    "Query params - path without query",
			path:    "/dashboard",
			pattern: "^/dashboard$",
			want:    true,
		},

		// Edge cases
		{
			name:    "Edge case - empty path",
			path:    "",
			pattern: "^/$",
			want:    false,
		},
		{
			name:    "Edge case - double slash",
			path:    "//dashboard",
			pattern: "^/dashboard$",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &setup.Config{
				AllowedRedirectPathPattern: tt.pattern,
			}
			got := isAllowedRedirectPath(tt.path, cfg)
			if got != tt.want {
				t.Errorf("isAllowedRedirectPath() = %v, want %v (path: %q, pattern: %q)",
					got, tt.want, tt.path, tt.pattern)
			}
		})
	}
}

func Test_isAllowedRedirectPath_InvalidRegex(t *testing.T) {
	cfg := &setup.Config{
		AllowedRedirectPathPattern: "^/dashboard[", // Invalid regex
	}

	// Should return false for invalid regex (error logged)
	got := isAllowedRedirectPath("/dashboard", cfg)
	if got != false {
		t.Errorf("isAllowedRedirectPath() with invalid regex = %v, want false", got)
	}
}
