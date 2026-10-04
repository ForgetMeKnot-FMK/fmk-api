package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

/*
What changed for the user after revision `since`, up to `revision` (the
user's current one). Attributes are small, so they are sent in full. Knots
are sent as ids; the client fetches the ones it needs.
*/
type syncResponse struct {
	Revision       int64           `json:"revision"`
	KnotAttributes []KnotAttribute `json:"knot_attributes"`
	Knots          []pgtype.UUID   `json:"knots"`
	Deleted        []pgtype.UUID   `json:"deleted"`
	UserAttributes *UserAttribute  `json:"user_attributes"` // nil when unchanged
}

// GET /sync?since=r
func (apiCfg *apiConfig) handleSync(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	since, err := strconv.ParseInt(c.QueryParam("since"), 10, 64)
	if err != nil || since < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "since must be a revision (integer >= 0)")
	}

	ctx := c.Request().Context()
	res := syncResponse{KnotAttributes: []KnotAttribute{}, Knots: []pgtype.UUID{}, Deleted: []pgtype.UUID{}}

	// One snapshot: the revision and the changes listed agree, even with writes in flight
	err = apiCfg.readSnapshot(ctx, func(q *database.Queries) error {
		var err error
		if res.Revision, err = q.GetUserRevision(ctx, user.ID); err != nil {
			return err
		}
		if res.Revision <= since {
			return nil // up to date
		}

		attrs, err := q.GetChangedKnotAttributes(ctx, database.GetChangedKnotAttributesParams{OwnerID: user.ID, Since: since})
		if err != nil {
			return err
		}
		for _, a := range attrs {
			res.KnotAttributes = append(res.KnotAttributes, databaseKnotAttributeToKnotAttribute(a))
		}

		changed := database.GetChangedKnotIDsParams{OwnerID: user.ID, Since: since}
		if ids, err := q.GetChangedKnotIDs(ctx, changed); err != nil {
			return err
		} else if ids != nil {
			res.Knots = ids
		}
		deleted := database.GetDeletedKnotIDsParams{OwnerID: user.ID, Since: since}
		if ids, err := q.GetDeletedKnotIDs(ctx, deleted); err != nil {
			return err
		} else if ids != nil {
			res.Deleted = ids
		}

		ua, err := q.GetUserAttributes(ctx, user.ID)
		if err == nil && ua.Revision > since {
			out := databaseUserAttributeToUserAttribute(ua)
			res.UserAttributes = &out
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot sync: %v", err))
	}
	return c.JSON(http.StatusOK, res)
}
