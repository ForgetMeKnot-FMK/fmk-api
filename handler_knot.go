package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

/**
List of things to do here:
	1. Make a handler function for creating a new knot
	2. Make a utility function for creating knot attributes
	3. Make a handler function to get the knot count
	4. Make a handler function to get all knots, paginated
	5. Make sure resources are behind auth checks
	6. Update knot + knot attributes
	7. (not in this file) Make seeders
*/

const defaultKeyVersion int32 = 1

func (apiCfg *apiConfig) handleKnotCreation(c *echo.Context) error {
	type parameters struct {
		// OwnerID            pgtype.UUID `json:"owner_id"`
		EncryptedContent   []byte `json:"encrypted_content"`
		EncryptedContentIv []byte `json:"encrypted_content_iv"`
		WrappedKnotKey     []byte `json:"wrapped_knot_key"`
		WrappedKnotKeyIv   []byte `json:"wrapped_knot_key_iv"`
		EncryptedTitle     []byte `json:"encrypted_title"`
		EncryptedTitleIv   []byte `json:"encrypted_title_iv"`
		// KeyVersion         []byte `json:"key_version"`
	}

	r := c.Request()

	user, err := apiCfg.getUserFromRequest(c)

	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized 2: %v", err))
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}

	// Todo: Validation (use `https://github.com/go-ozzo/ozzo-validation`)

	var knot database.Knot
	_, err = apiCfg.withRevision(r.Context(), user.ID, func(q *database.Queries, rev int64) (err error) {
		knot, err = q.CreateKnot(r.Context(), database.CreateKnotParams{
			OwnerID:            user.ID,
			EncryptedContent:   params.EncryptedContent,
			EncryptedContentIv: params.EncryptedContentIv,
			WrappedKnotKey:     params.WrappedKnotKey,
			WrappedKnotKeyIv:   params.WrappedKnotKeyIv,
			EncryptedTitle:     params.EncryptedTitle,
			EncryptedTitleIv:   params.EncryptedTitleIv,
			KeyVersion:         defaultKeyVersion,
			Revision:           rev,
		})
		return err
	})

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot create knot: %v", err))
	}

	return c.JSON(http.StatusCreated, databaseKnotToKnot(knot))
}

func (apiCfg *apiConfig) handleKnotUpdate(c *echo.Context) error {
	type parameters struct {
		EncryptedContent   []byte `json:"encrypted_content"`
		EncryptedContentIv []byte `json:"encrypted_content_iv"`
		WrappedKnotKey     []byte `json:"wrapped_knot_key"`
		WrappedKnotKeyIv   []byte `json:"wrapped_knot_key_iv"`
		EncryptedTitle     []byte `json:"encrypted_title"`
		EncryptedTitleIv   []byte `json:"encrypted_title_iv"`
	}

	r := c.Request()

	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}

	var knotID pgtype.UUID
	if err := knotID.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}

	params := parameters{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}

	// Todo: Validation (use `https://github.com/go-ozzo/ozzo-validation`)

	var knot database.Knot
	_, err = apiCfg.withRevision(r.Context(), user.ID, func(q *database.Queries, rev int64) (err error) {
		knot, err = q.UpdateKnot(r.Context(), database.UpdateKnotParams{
			ID:                 knotID,
			OwnerID:            user.ID,
			EncryptedContent:   params.EncryptedContent,
			EncryptedContentIv: params.EncryptedContentIv,
			WrappedKnotKey:     params.WrappedKnotKey,
			WrappedKnotKeyIv:   params.WrappedKnotKeyIv,
			EncryptedTitle:     params.EncryptedTitle,
			EncryptedTitleIv:   params.EncryptedTitleIv,
			UpdatedAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
			Revision:           rev,
		})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Missing, deleted, or owned by someone else; keep "which" a secret
		return echo.NewHTTPError(http.StatusNotFound, "Knot not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot update knot: %v", err))
	}

	return c.JSON(http.StatusOK, databaseKnotToKnot(knot))
}

func (apiCfg *apiConfig) handleKnotGet(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	var knotID pgtype.UUID
	if err := knotID.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}

	knot, err := apiCfg.DB.GetKnotByIDAndOwner(c.Request().Context(), database.GetKnotByIDAndOwnerParams{ID: knotID, OwnerID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Knot not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot get knot: %v", err))
	}
	return c.JSON(http.StatusOK, databaseKnotToKnot(knot))
}

func (apiCfg *apiConfig) handleKnotDelete(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var knotID pgtype.UUID
	if err := knotID.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}

	/*
		Soft delete: the row stays, with `deleted_revision` set, so other
		devices can learn about it through GET /sync. Its attributes stay too,
		but are hidden.
	*/
	ctx := c.Request().Context()
	rev, err := apiCfg.withRevision(ctx, user.ID, func(q *database.Queries, rev int64) error {
		rows, err := q.SoftDeleteKnot(ctx, database.SoftDeleteKnotParams{ID: knotID, OwnerID: user.ID, Revision: rev})
		if err == nil && rows == 0 {
			return pgx.ErrNoRows // missing or already deleted: roll back, keep the revision unused
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Knot not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot delete knot")
	}
	return c.JSON(http.StatusOK, map[string]int64{"revision": rev})
}

/**
*************************************************************
********************** Knot Attributes **********************
*************************************************************
**/

// Shared by POST and PUT: auth, knot ID, body, basic validation
func (apiCfg *apiConfig) parseKnotAttributesRequest(c *echo.Context) (pgtype.UUID, pgtype.UUID, attributesParams, error) {
	var knotID pgtype.UUID
	params := attributesParams{}

	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return knotID, pgtype.UUID{}, params, echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	if err := knotID.Scan(c.Param("id")); err != nil {
		return knotID, user.ID, params, echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}
	params, err = parseAttributesBody(c)
	return knotID, user.ID, params, err
}

func (apiCfg *apiConfig) handleKnotAttributesCreation(c *echo.Context) error {
	knotID, ownerID, params, err := apiCfg.parseKnotAttributesRequest(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	var attr database.KnotAttribute
	_, err = apiCfg.withRevision(ctx, ownerID, func(q *database.Queries, rev int64) (err error) {
		attr, err = q.CreateKnotAttributes(ctx, database.CreateKnotAttributesParams{
			KnotID:     knotID,
			OwnerID:    ownerID,
			Data:       params.Data,
			DataIv:     params.DataIv,
			KeyVersion: defaultKeyVersion,
			Revision:   rev,
		})
		return err
	})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows): // knot missing, deleted, or not owned by the user
		return echo.NewHTTPError(http.StatusNotFound, "Knot not found")
	case errors.As(err, &pgErr) && pgErr.Code == "23505": // unique_violation
		return echo.NewHTTPError(http.StatusConflict, "Attributes already exist; use PUT")
	case err != nil:
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			fmt.Sprintf("Cannot create knot attributes: %v", err),
		)
	}

	return c.JSON(http.StatusCreated, databaseKnotAttributeToKnotAttribute(attr))
}

func (apiCfg *apiConfig) handleKnotAttributesUpdate(c *echo.Context) error {
	knotID, ownerID, params, err := apiCfg.parseKnotAttributesRequest(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	var attr database.KnotAttribute
	_, err = apiCfg.withRevision(ctx, ownerID, func(q *database.Queries, rev int64) (err error) {
		attr, err = q.UpdateKnotAttributes(ctx, database.UpdateKnotAttributesParams{
			KnotID:   knotID,
			OwnerID:  ownerID,
			Data:     params.Data,
			DataIv:   params.DataIv,
			Revision: rev,
		})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Knot attributes not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot update knot attributes: %v", err))
	}

	return c.JSON(http.StatusOK, databaseKnotAttributeToKnotAttribute(attr))
}

func (apiCfg *apiConfig) handleKnotAttributesGet(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	var knotID pgtype.UUID
	if err := knotID.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}

	attr, err := apiCfg.DB.GetKnotAttributesByKnotID(c.Request().Context(), database.GetKnotAttributesByKnotIDParams{KnotID: knotID, OwnerID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Knot attributes not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot get knot attributes: %v", err))
	}
	return c.JSON(http.StatusOK, databaseKnotAttributeToKnotAttribute(attr))
}

func (apiCfg *apiConfig) handleKnotAttributesList(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}

	rows, err := apiCfg.DB.GetKnotAttributesByOwnerID(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot list knot attributes: %v", err))
	}
	out := make([]KnotAttribute, 0, len(rows)) // [] rather than null when empty
	for _, r := range rows {
		out = append(out, databaseKnotAttributeToKnotAttribute(r))
	}
	return c.JSON(http.StatusOK, out)
}
