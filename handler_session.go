package main

import (
	"net/http"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

/*
Device sessions: the user can see every live session of the account, and end
any of them. A lost device's session can then be ended from another device,
so the master key cached on it is not enough to reach the user's data.

Ending a session sets revoked_at. The token lookups skip revoked sessions, so
the token gets 401 on its next request.
*/

// GET /user: who is logged in
func (apiCfg *apiConfig) handleProfileGet(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return c.JSON(http.StatusOK, databaseUserToProfile(user))
}

// GET /sessions: the user's live sessions, most recently used first
func (apiCfg *apiConfig) handleSessionsList(c *echo.Context) error {
	auth, err := apiCfg.getAuthFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	rows, err := apiCfg.DB.GetSessionsByUserID(c.Request().Context(), auth.User.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot list sessions")
	}
	out := make([]Session, 0, len(rows)) // [] rather than null when empty
	for _, s := range rows {
		out = append(out, databaseSessionToSession(s, auth.SessionID))
	}
	return c.JSON(http.StatusOK, out)
}

// DELETE /sessions/:id: ends one of the user's sessions (404 for anyone else's)
func (apiCfg *apiConfig) handleSessionRevoke(c *echo.Context) error {
	auth, err := apiCfg.getAuthFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}
	return apiCfg.revoke(c, id, auth.User.ID)
}

// DELETE /sessions: ends all of the user's sessions except the current one
func (apiCfg *apiConfig) handleOtherSessionsRevoke(c *echo.Context) error {
	auth, err := apiCfg.getAuthFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	n, err := apiCfg.DB.RevokeOtherSessions(c.Request().Context(), database.RevokeOtherSessionsParams{UserID: auth.User.ID, ID: auth.SessionID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot end sessions")
	}
	return c.JSON(http.StatusOK, map[string]int64{"ended": n})
}

// POST /logout: ends the current session
func (apiCfg *apiConfig) handleLogout(c *echo.Context) error {
	auth, err := apiCfg.getAuthFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	return apiCfg.revoke(c, auth.SessionID, auth.User.ID)
}

// Ends a session of the user; 404 if it isn't theirs or has already ended
func (apiCfg *apiConfig) revoke(c *echo.Context, id, userID pgtype.UUID) error {
	n, err := apiCfg.DB.RevokeSession(c.Request().Context(), database.RevokeSessionParams{ID: id, UserID: userID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot end the session")
	}
	if n == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Session not found")
	}
	return c.NoContent(http.StatusNoContent)
}
