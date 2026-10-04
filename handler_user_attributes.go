package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
)

/**
User attributes: one client-encrypted JSON blob per user (e.g. labels and categories for knots),
synced between the user's devices. Writes are optimistic: a PUT names the revision it was based
on, and a stale one gets 409 with the current row, for the client to merge onto and retry.
*/

// A PUT based on an outdated revision; carries the current row for the client
type staleUserAttributesError struct {
	current database.UserAttribute
}

func (e *staleUserAttributesError) Error() string { return "stale user attributes revision" }

func (apiCfg *apiConfig) handleUserAttributesGet(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}

	attr, err := apiCfg.DB.GetUserAttributes(c.Request().Context(), user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "User attributes not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot get user attributes: %v", err))
	}
	return c.JSON(http.StatusOK, databaseUserAttributeToUserAttribute(attr))
}

func (apiCfg *apiConfig) handleUserAttributesCreation(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	params, err := parseAttributesBody(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	var attr database.UserAttribute
	_, err = apiCfg.withRevision(ctx, user.ID, func(q *database.Queries, rev int64) (err error) {
		attr, err = q.CreateUserAttributes(ctx, database.CreateUserAttributesParams{
			UserID:     user.ID,
			Data:       params.Data,
			DataIv:     params.DataIv,
			KeyVersion: defaultKeyVersion,
			Revision:   rev,
		})
		return err
	})
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505": // unique_violation
		return echo.NewHTTPError(http.StatusConflict, "User attributes already exist; use PUT")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot create user attributes: %v", err))
	}

	return c.JSON(http.StatusCreated, databaseUserAttributeToUserAttribute(attr))
}

func (apiCfg *apiConfig) handleUserAttributesUpdate(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	params, err := parseAttributesBody(c)
	if err != nil {
		return err
	}
	if params.Revision == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "revision required (the version this update is based on)")
	}

	ctx := c.Request().Context()
	var attr database.UserAttribute
	_, err = apiCfg.withRevision(ctx, user.ID, func(q *database.Queries, rev int64) error {
		updated, err := q.UpdateUserAttributes(ctx, database.UpdateUserAttributesParams{
			UserID:           user.ID,
			Data:             params.Data,
			DataIv:           params.DataIv,
			Revision:         rev,
			ExpectedRevision: *params.Revision,
		})
		if !errors.Is(err, pgx.ErrNoRows) {
			attr = updated
			return err
		}
		// No row updated: missing (404), or another device changed it first (409)
		current, err := q.GetUserAttributes(ctx, user.ID)
		if err != nil {
			return err // ErrNoRows: missing
		}
		return &staleUserAttributesError{current: current}
	})

	var stale *staleUserAttributesError
	switch {
	case errors.As(err, &stale):
		return c.JSON(http.StatusConflict, databaseUserAttributeToUserAttribute(stale.current))
	case errors.Is(err, pgx.ErrNoRows):
		return echo.NewHTTPError(http.StatusNotFound, "User attributes not found; use POST")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot update user attributes: %v", err))
	}

	return c.JSON(http.StatusOK, databaseUserAttributeToUserAttribute(attr))
}
