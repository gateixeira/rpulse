package utils

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gateixeira/rpulse/models"
)

// Contains checks if a string is present in a slice
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetRunnerType determines if a runner is self-hosted or GitHub-hosted based on its labels
func GetRunnerType(labels []string) models.RunnerType {
	if Contains(labels, "self-hosted") {
		return models.RunnerTypeSelfHosted
	}

	return models.RunnerTypeGitHubHosted
}

// GenerateCSRFToken generates a random token for CSRF protection
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CookieName is the name of the CSRF cookie
const CookieName = "csrf_token"

// HeaderName is the name of the CSRF header
const HeaderName = "X-CSRF-Token"
