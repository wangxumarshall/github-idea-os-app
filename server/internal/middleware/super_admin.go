package middleware

import (
	"net/http"
	"strings"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// RequireSuperAdmin restricts access to the configured super admin account.
func RequireSuperAdmin(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
			if userID == "" {
				http.Error(w, `{"error":"user not authenticated"}`, http.StatusUnauthorized)
				return
			}

			email := strings.TrimSpace(r.Header.Get("X-User-Email"))
			if email == "" && queries != nil {
				user, err := queries.GetUser(r.Context(), util.ParseUUID(userID))
				if err == nil {
					email = user.Email
				}
			}

			if !auth.IsSuperAdminEmail(email) {
				http.Error(w, `{"error":"super admin access required"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
