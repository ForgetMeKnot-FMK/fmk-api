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

	knotParams := database.CreateKnotParams{
		OwnerID:            user.ID,
		EncryptedContent:   params.EncryptedContent,
		EncryptedContentIv: params.EncryptedContentIv,
		WrappedKnotKey:     params.WrappedKnotKey,
		WrappedKnotKeyIv:   params.WrappedKnotKeyIv,
		EncryptedTitle:     params.EncryptedTitle,
		EncryptedTitleIv:   params.EncryptedTitleIv,
		KeyVersion:         defaultKeyVersion,
	}

	knot, err := apiCfg.DB.CreateKnot(r.Context(), knotParams)

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

	knot, err := apiCfg.DB.UpdateKnot(r.Context(), database.UpdateKnotParams{
		ID:                 knotID,
		OwnerID:            user.ID,
		EncryptedContent:   params.EncryptedContent,
		EncryptedContentIv: params.EncryptedContentIv,
		WrappedKnotKey:     params.WrappedKnotKey,
		WrappedKnotKeyIv:   params.WrappedKnotKeyIv,
		EncryptedTitle:     params.EncryptedTitle,
		EncryptedTitleIv:   params.EncryptedTitleIv,
		UpdatedAt:          pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Missing or owned by someone else; don't reveal which
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

/**
*************************************************************
********************** Knot Attributes **********************
*************************************************************
**/

type knotAttributesParams struct {
	Data   []byte `json:"data"`
	DataIv []byte `json:"data_iv"`
}

// Shared by POST and PUT: auth, knot ID, body, basic validation
func (apiCfg *apiConfig) parseKnotAttributesRequest(c *echo.Context) (pgtype.UUID, pgtype.UUID, knotAttributesParams, error) {
	var knotID pgtype.UUID
	params := knotAttributesParams{}

	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return knotID, pgtype.UUID{}, params, echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	if err := knotID.Scan(c.Param("id")); err != nil {
		return knotID, user.ID, params, echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&params); err != nil {
		return knotID, user.ID, params, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}
	if len(params.Data) == 0 || len(params.DataIv) != 24 {
		return knotID, user.ID, params, echo.NewHTTPError(http.StatusBadRequest, "data required; data_iv must be 24 bytes")
	}
	return knotID, user.ID, params, nil
}

func (apiCfg *apiConfig) handleKnotAttributesCreation(c *echo.Context) error {
	knotID, ownerID, params, err := apiCfg.parseKnotAttributesRequest(c)
	if err != nil {
		return err
	}

	attr, err := apiCfg.DB.CreateKnotAttributes(
		c.Request().Context(),
		database.CreateKnotAttributesParams{
			KnotID:     knotID,
			OwnerID:    ownerID,
			Data:       params.Data,
			DataIv:     params.DataIv,
			KeyVersion: defaultKeyVersion,
		})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows): // knot missing or not owned by the user
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

	attr, err := apiCfg.DB.UpdateKnotAttributes(
		c.Request().Context(),
		database.UpdateKnotAttributesParams{
			KnotID:  knotID,
			OwnerID: ownerID,
			Data:    params.Data,
			DataIv:  params.DataIv,
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
