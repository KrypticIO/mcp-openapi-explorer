package github

import (
	"fmt"
	"strings"
)

// IsGitHubURL checks if a URL is a GitHub URL
func IsGitHubURL(url string) bool {
	return strings.HasPrefix(url, "github.com") ||
		strings.HasPrefix(url, "@github.com") ||
		strings.HasPrefix(url, "https://github.com") ||
		strings.HasPrefix(url, "http://github.com")
}

// TrimGitHubPrefix removes the "@" prefix from a GitHub URL if present
func TrimGitHubPrefix(url string) string {
	return strings.TrimPrefix(url, "@")
}

// ConvertGitHubURLToRaw converts a GitHub URL to a raw.githubusercontent.com URL
// If token is provided, it uses it for private repositories
func ConvertGitHubURLToRaw(githubURL, token string) (string, error) {
	// Remove any protocol prefix if present
	githubURL = strings.TrimPrefix(githubURL, "https://")
	githubURL = strings.TrimPrefix(githubURL, "http://")

	// Ensure it's a GitHub URL
	if !strings.HasPrefix(githubURL, "github.com") {
		return "", fmt.Errorf("not a GitHub URL: %s", githubURL)
	}

	// Handle GitHub path
	parts := strings.Split(githubURL, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid GitHub URL format: %s", githubURL)
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

	// Construct raw URL
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)

	// Add token if provided
	if token != "" {
		rawURL = fmt.Sprintf("https://%s@raw.githubusercontent.com/%s/%s/%s/%s", token, owner, repo, branch, path)
	}

	return rawURL, nil
}
