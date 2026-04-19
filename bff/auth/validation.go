package auth

import (
	"bff/setup"
	"log/slog"
	"regexp"
)

// isAllowedRedirectPath validates if the given path is allowed based on the configured pattern.
// Returns true if the path is allowed, false otherwise.
func isAllowedRedirectPath(path string, cfg *setup.Config) bool {
	// If no pattern is configured, allow all paths (backward compatibility)
	if cfg.AllowedRedirectPathPattern == "" {
		return true
	}

	// Match against the configured regex pattern
	matched, err := regexp.MatchString(cfg.AllowedRedirectPathPattern, path)
	if err != nil {
		// This should be caught by startup validation, but log just in case
		slog.Error("Invalid redirect path pattern", "pattern", cfg.AllowedRedirectPathPattern, "error", err)
		return false
	}
	return matched
}
