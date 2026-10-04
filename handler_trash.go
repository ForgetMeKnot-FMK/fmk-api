package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

// GET /knot/trash: the attributes of the user's deleted knots, last deleted first
func (apiCfg *apiConfig) handleTrashList(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}

	rows, err := apiCfg.DB.GetDeletedKnotAttributes(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot list the trash")
	}
	out := make([]KnotAttribute, 0, len(rows)) // [] rather than null when empty
	for _, r := range rows {
		out = append(out, databaseKnotAttributeToKnotAttribute(r))
	}
	return c.JSON(http.StatusOK, out)
}

type restoreResponse struct {
	Revision       int64          `json:"revision"`
	KnotAttributes *KnotAttribute `json:"knot_attributes"` // null if the knot has none
}

/*
POST /knot/:id/restore: takes a knot out of the trash.

 1. In one revision: clear the knot's `deleted_revision`, and stamp the new
    revision on the knot and on its attributes.
 2. GET /sync then lists the knot and its attributes again, so other devices
    show it again.
 3. Answer with the revision and the attributes, so this device can show the
    card without a sync.

Example: the knot was deleted at revision 40. Restoring it at revision 45 sets
the knot's and the attributes' revision to 45. A device at 42 gets both rows
from GET /sync?since=42.
*/
func (apiCfg *apiConfig) handleKnotRestore(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	var knotID pgtype.UUID
	if err := knotID.Scan(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid knot ID")
	}

	ctx := c.Request().Context()
	var attrs *KnotAttribute
	rev, err := apiCfg.withRevision(ctx, user.ID, func(q *database.Queries, rev int64) error {
		rows, err := q.RestoreKnot(ctx, database.RestoreKnotParams{ID: knotID, OwnerID: user.ID, Revision: rev})
		if err != nil {
			return err
		}
		if rows == 0 {
			return pgx.ErrNoRows // missing, or not deleted: roll back
		}
		row, err := q.StampKnotAttributes(ctx, database.StampKnotAttributesParams{KnotID: knotID, OwnerID: user.ID, Revision: rev})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // a knot without attributes
		}
		if err != nil {
			return err
		}
		a := databaseKnotAttributeToKnotAttribute(row)
		attrs = &a
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Knot not found in the trash")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot restore knot")
	}
	return c.JSON(http.StatusOK, restoreResponse{Revision: rev, KnotAttributes: attrs})
}
