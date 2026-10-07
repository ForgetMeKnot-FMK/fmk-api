package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

/*
Encrypted blobs (images in knots). The client encrypts each one with its own key, wraps that
key with the user's master key, and uploads the bytes with PUT /blob/:id. The server keeps the
bytes on disk (see blob_store.go) and the wrapped key in the `blobs` table. It never sees the
content. Knots say which blobs they use (`blob_ids`, see setKnotBlobs).
*/

// The largest blob accepted (encrypted size)
const maxBlobSize = 10 << 20

/*
The per-knot quota: how many blobs one knot may use, and their total encrypted size.
KNOT_BLOB_MAX_COUNT (default 101: the web client keeps one for a knot's cover, a thumbnail of
its first image, so 100 images) and KNOT_BLOB_QUOTA_MB (default 25). A knot write that adds
blobs beyond it is refused (413). One that adds none goes through, so a knot already over (the
quota was lowered) can still be edited.
*/
type blobQuota struct {
	Count int64 `json:"knot_count"`
	Bytes int64 `json:"knot_bytes"`
}

// The knot's blobs would go over the quota
type quotaError struct{ quota blobQuota }

func (e *quotaError) Error() string { return "knot blob quota exceeded" }

// GET /blob/quota: the per-knot quota, so the client can check before adding an image
func (apiCfg *apiConfig) handleBlobQuota(c *echo.Context) error {
	if _, err := apiCfg.getUserFromRequest(c); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	return c.JSON(http.StatusOK, apiCfg.Quota)
}

// Answers a knot write refused by the quota (413, with the quota)
func quotaResponse(c *echo.Context, err error) (bool, error) {
	var over *quotaError
	if !errors.As(err, &over) {
		return false, nil
	}
	return true, c.JSON(http.StatusRequestEntityTooLarge, map[string]any{
		"message": "The knot's images are over its quota",
		"quota":   over.quota,
	})
}

// What the client needs to open a blob: its wrapped key (the bytes come from GET /blob/:id)
type BlobInfo struct {
	ID           pgtype.UUID `json:"id"`
	WrappedKey   []byte      `json:"wrapped_key"`
	WrappedKeyIv []byte      `json:"wrapped_key_iv"`
	Size         int64       `json:"size"`
}

func databaseBlobToBlobInfo(b database.Blob) BlobInfo {
	return BlobInfo{ID: b.ID, WrappedKey: b.WrappedKey, WrappedKeyIv: b.WrappedKeyIv, Size: b.Size}
}

// Reads a base64 header of an exact length (in bytes, after decoding)
func base64Header(c *echo.Context, name string, length int) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(c.Request().Header.Get(name))
	if err != nil || len(b) != length {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("%s: %d bytes of base64 expected", name, length))
	}
	return b, nil
}

func blobIDParam(c *echo.Context) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		return id, echo.NewHTTPError(http.StatusBadRequest, "Invalid blob ID")
	}
	return id, nil
}

/*
handleBlobUpload stores a new blob (PUT /blob/:id, the encrypted bytes as the body).

Headers (base64): X-Blob-Key (the wrapped key, 48 bytes), X-Blob-Key-Iv (24 bytes),
X-Blob-Sha256 (of the body, 32 bytes) and, optionally, X-Blob-Tag (the dedup tag, 32 bytes).

Steps:
 1. Read the body (at most 10 MiB) into a temporary file, hashing it on the way. A hash that
    doesn't match the header is refused.
 2. Move the file into place, then add the row (unreferenced, until a knot uses it).
 3. The user has a blob with this tag already: 409 with it (the file is removed). The same
    upload again (a retry: same id, owner and hash): 200 with it. Another blob with this id:
    409 without details.

Example: a phone uploads a photo; the same photo from the same account again finds it by
its tag first (GET /blob?tag=), and isn't uploaded twice.
*/
func (apiCfg *apiConfig) handleBlobUpload(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	id, err := blobIDParam(c)
	if err != nil {
		return err
	}
	wrappedKey, err := base64Header(c, "X-Blob-Key", 48)
	if err != nil {
		return err
	}
	wrappedKeyIv, err := base64Header(c, "X-Blob-Key-Iv", 24)
	if err != nil {
		return err
	}
	sum, err := base64Header(c, "X-Blob-Sha256", 32)
	if err != nil {
		return err
	}
	tag := []byte(nil)
	if c.Request().Header.Get("X-Blob-Tag") != "" {
		if tag, err = base64Header(c, "X-Blob-Tag", 32); err != nil {
			return err
		}
	}

	body := http.MaxBytesReader(c.Response(), c.Request().Body, maxBlobSize)
	tmp, size, got, err := apiCfg.Blobs.writeTemp(body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "Blob too large (10 MiB at most)")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot store blob: %v", err))
	}
	defer os.Remove(tmp) // no-op once moved into place
	if !bytes.Equal(got, sum) {
		return echo.NewHTTPError(http.StatusBadRequest, "X-Blob-Sha256 doesn't match the body")
	}

	ctx := c.Request().Context()
	// An earlier try of this same upload may have stored it already
	if existing, err := apiCfg.DB.GetBlob(ctx, id); err == nil {
		if existing.OwnerID == user.ID && bytes.Equal(existing.Sha256, sum) {
			return c.JSON(http.StatusOK, databaseBlobToBlobInfo(existing))
		}
		return echo.NewHTTPError(http.StatusConflict, "Blob ID taken")
	}

	if err := apiCfg.Blobs.place(tmp, id); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot store blob: %v", err))
	}
	blob, err := apiCfg.DB.CreateBlob(ctx, database.CreateBlobParams{
		ID:           id,
		OwnerID:      user.ID,
		WrappedKey:   wrappedKey,
		WrappedKeyIv: wrappedKeyIv,
		Size:         size,
		Sha256:       sum,
		DedupTag:     tag,
	})
	if err != nil {
		apiCfg.Blobs.remove(id)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && tag != nil { // unique_violation
			existing, getErr := apiCfg.DB.GetBlobByTag(ctx, database.GetBlobByTagParams{OwnerID: user.ID, DedupTag: tag})
			if getErr == nil {
				return c.JSON(http.StatusConflict, databaseBlobToBlobInfo(existing))
			}
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return echo.NewHTTPError(http.StatusConflict, "Blob ID taken")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot store blob: %v", err))
	}
	return c.JSON(http.StatusCreated, databaseBlobToBlobInfo(blob))
}

// GET /blob?tag=<base64>: the user's blob made from the same file, if there is one
func (apiCfg *apiConfig) handleBlobFind(c *echo.Context) error {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	tag, err := base64.StdEncoding.DecodeString(c.QueryParam("tag"))
	if err != nil || len(tag) != 32 {
		return echo.NewHTTPError(http.StatusBadRequest, "tag: 32 bytes of base64 expected")
	}
	blob, err := apiCfg.DB.GetBlobByTag(c.Request().Context(), database.GetBlobByTagParams{OwnerID: user.ID, DedupTag: tag})
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "Blob not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot find blob: %v", err))
	}
	return c.JSON(http.StatusOK, databaseBlobToBlobInfo(blob))
}

// The user's own blob, or a 404 (also for anyone else's: which ids exist stays a secret)
func (apiCfg *apiConfig) ownBlob(c *echo.Context) (database.Blob, error) {
	user, err := apiCfg.getUserFromRequest(c)
	if err != nil {
		return database.Blob{}, echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err))
	}
	id, err := blobIDParam(c)
	if err != nil {
		return database.Blob{}, err
	}
	blob, err := apiCfg.DB.GetBlobByOwner(c.Request().Context(), database.GetBlobByOwnerParams{ID: id, OwnerID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return blob, echo.NewHTTPError(http.StatusNotFound, "Blob not found")
	}
	if err != nil {
		return blob, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot get blob: %v", err))
	}
	return blob, nil
}

// GET /blob/:id: the encrypted bytes. A blob never changes, so browsers may keep it
func (apiCfg *apiConfig) handleBlobGet(c *echo.Context) error {
	blob, err := apiCfg.ownBlob(c)
	if err != nil {
		return err
	}
	f, err := apiCfg.Blobs.open(blob.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot read blob: %v", err))
	}
	defer f.Close()
	c.Response().Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	c.Response().Header().Set("Content-Length", fmt.Sprint(blob.Size))
	return c.Stream(http.StatusOK, "application/octet-stream", f)
}

// GET /blob/:id/key: the wrapped key, for a device that hasn't seen the blob yet
func (apiCfg *apiConfig) handleBlobKey(c *echo.Context) error {
	blob, err := apiCfg.ownBlob(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, databaseBlobToBlobInfo(blob))
}

/*
setKnotBlobs makes a knot's `knot_blobs` rows match the blobs it uses now, inside the knot
write's transaction.

Steps:
 1. If `ids` adds blobs the knot doesn't use yet, check the per-knot quota on the whole set
    (count and total size): over it, refuse (quotaError).
 2. Remove the rows of blobs not in `ids`. Those no other knot uses become unreferenced
    (`unreferenced_since` = now): the cleanup job deletes them after a while.
 3. Add rows for the new ones (only blobs the user owns; others are skipped).
 4. The blobs in `ids` are in use: `unreferenced_since` = NULL.

Example: a knot used images a and b, and now uses b and c. a loses its row (and, if no other
knot uses it, starts its 30 days); c gets a row.
*/
func setKnotBlobs(q *database.Queries, ctx echoContext, quota blobQuota, knotID, ownerID pgtype.UUID, ids []pgtype.UUID) error {
	if err := checkKnotQuota(q, ctx, quota, knotID, ownerID, ids); err != nil {
		return err
	}
	removed, err := q.RemoveKnotBlobsNotIn(ctx, database.RemoveKnotBlobsNotInParams{KnotID: knotID, BlobIds: ids})
	if err != nil {
		return err
	}
	if len(removed) > 0 {
		if err := q.MarkBlobsUnreferenced(ctx, removed); err != nil {
			return err
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if err := q.AddKnotBlobs(ctx, database.AddKnotBlobsParams{KnotID: knotID, BlobIds: ids, OwnerID: ownerID}); err != nil {
		return err
	}
	return q.MarkBlobsReferenced(ctx, database.MarkBlobsReferencedParams{BlobIds: ids, OwnerID: ownerID})
}

// Refuses a set of blobs over the quota, when it adds any the knot doesn't use yet
func checkKnotQuota(q *database.Queries, ctx echoContext, quota blobQuota, knotID, ownerID pgtype.UUID, ids []pgtype.UUID) error {
	linked, err := q.KnotBlobIDs(ctx, knotID)
	if err != nil {
		return err
	}
	has := map[string]bool{}
	for _, id := range linked {
		has[id.String()] = true
	}
	adds := false
	for _, id := range ids {
		adds = adds || !has[id.String()]
	}
	if !adds {
		return nil
	}
	total, err := q.BlobsTotal(ctx, database.BlobsTotalParams{Ids: ids, OwnerID: ownerID})
	if err != nil {
		return err
	}
	if total.Count > quota.Count || total.Size > quota.Bytes {
		return &quotaError{quota: quota}
	}
	return nil
}

// The most blob ids one knot may list (more than any quota allows)
const maxKnotBlobs = 1000

// Parses a knot write's `blob_ids` (each once; nil when the client didn't send any)
func parseBlobIDs(raw *[]string) ([]pgtype.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	if len(*raw) > maxKnotBlobs {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("blob_ids: %d at most", maxKnotBlobs))
	}
	seen := map[string]bool{}
	ids := make([]pgtype.UUID, 0, len(*raw))
	for _, s := range *raw {
		var id pgtype.UUID
		if err := id.Scan(s); err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "blob_ids: invalid id")
		}
		if !seen[id.String()] {
			seen[id.String()] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}
