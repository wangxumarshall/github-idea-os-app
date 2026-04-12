package auth

import (
	"os"
	"strings"
)

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func SuperAdminEmail() string {
	return NormalizeEmail(os.Getenv("SUPER_ADMIN_EMAIL"))
}

func IsSuperAdminEmail(email string) bool {
	superAdminEmail := SuperAdminEmail()
	return superAdminEmail != "" && NormalizeEmail(email) == superAdminEmail
}
