package github

import (
	"errors"
	"fmt"
	"strings"
)

// Common errors for GitHub operations
var (
	ErrInvalidGitHubURL = errors.New("invalid GitHub URL")
	ErrNotGitHubURL     = errors.New("not a GitHub URL")
)

// IsGitHubURL checks if a URL is a GitHub URL
func IsGitHubURL(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "github.com") ||
		strings.HasPrefix(lowerURL, "@github.com")
}

// TrimGitHubPrefix removes the "@" prefix from a GitHub URL if present
func TrimGitHubPrefix(url string) string {
	return strings.TrimPrefix(url, "@")
}

// NormalizeGitHubURL removes protocol prefixes and ensures consistent format
func NormalizeGitHubURL(url string) string {
	// Remove protocol prefixes
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	// Remove @ prefix if present
	url = TrimGitHubPrefix(url)

	return url
}

// ConvertGitHubURLToRaw converts a GitHub URL to a raw.githubusercontent.com URL
// If token is provided, it uses it for private repositories
func ConvertGitHubURLToRaw(githubURL, token string) (string, error) {
	// Normalize the URL
	githubURL = NormalizeGitHubURL(githubURL)

	// Ensure it's a GitHub URL
	if !strings.HasPrefix(githubURL, "github.com") {
		return "", fmt.Errorf("%w: %s", ErrNotGitHubURL, githubURL)
	}

	// Handle GitHub path
	parts := strings.Split(githubURL, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("%w: insufficient path segments in %s (expected at least owner/repo/branch/path)",
			ErrInvalidGitHubURL, githubURL)
	}

	// Extract relevant parts
	owner := parts[1]
	repo := parts[2]

	// Determine if it's a blob URL or not
	var branch, path string
	if parts[3] == "blob" {
		branch = parts[4]
		path = strings.Join(parts[5:], "/")
	} else {
		branch = parts[3]
		path = strings.Join(parts[4:], "/")
	}

	// Validate parts
	if owner == "" || repo == "" || branch == "" {
		return "", fmt.Errorf("%w: missing owner, repo, or branch in %s",
			ErrInvalidGitHubURL, githubURL)
	}

	// Construct raw URL
	var rawURL string
	if token != "" {
		rawURL = fmt.Sprintf("https://%s@raw.githubusercontent.com/%s/%s/%s/%s",
			token, owner, repo, branch, path)
	} else {
		rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			owner, repo, branch, path)
	}

	return rawURL, nil
}
