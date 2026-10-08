package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/alexedwards/argon2id"
	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/labstack/echo/v5"
)

/*
handlePasswordChange is PUT /user/password: the logged-in user sets a new password.

The master key stays the same (no data changes): the client wraps it again with the key
the new password gives, and sends a new X25519 key pair (its private key wrapped the same
way). The server never sees either password.

 1. Check the session, and the current password's auth hash (403 if wrong: not 401, which
    would end the client's session).
 2. Hash the new auth hash as at registration.
 3. In one transaction: save the new hash, KDF settings and keys, and end the user's other
    sessions (a password change logs out the other devices).
 4. Answer with how many sessions were ended.

Example: {"current_auth_hash": "...", "authHash": "...", "kdf_salt": "...", ...} gives
{"ended": 2}.
*/
func (apiCfg *apiConfig) handlePasswordChange(c *echo.Context) error {
	type parameters struct {
		CurrentAuthHashRaw   []byte `json:"current_auth_hash"`
		AuthHashRaw          []byte `json:"authHash"`
		KDFSalt              []byte `json:"kdf_salt"`
		EncryptedMasterKey   []byte `json:"encrypted_master_key"`
		EncryptedMasterKeyIV []byte `json:"encrypted_master_key_iv"`
		PubKeyX25519         []byte `json:"pub_key_x25519"`
		EncryptedPrivKey     []byte `json:"encrypted_priv_key"`
		EncryptedPrivKeyIv   []byte `json:"encrypted_priv_key_iv"`
	}

	auth, err := apiCfg.getAuthFromRequest(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	r := c.Request()
	params := parameters{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}
	if len(params.CurrentAuthHashRaw) == 0 || len(params.AuthHashRaw) == 0 ||
		len(params.KDFSalt) != AuthHashSaltBytes || len(params.PubKeyX25519) != 32 ||
		len(params.EncryptedMasterKey) == 0 || len(params.EncryptedMasterKeyIV) == 0 ||
		len(params.EncryptedPrivKey) == 0 || len(params.EncryptedPrivKeyIv) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Missing or malformed fields")
	}

	match, err := argon2id.ComparePasswordAndHash(
		base64.StdEncoding.EncodeToString(params.CurrentAuthHashRaw), auth.User.AuthHash)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong, try again later")
	}
	if !match {
		return echo.NewHTTPError(http.StatusForbidden, "The current password is wrong")
	}

	authHash, err := argon2id.CreateHash(
		base64.StdEncoding.EncodeToString(params.AuthHashRaw), argon2id.DefaultParams)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Something went wrong, try again later")
	}

	ctx := r.Context()
	tx, err := apiCfg.Pool.Begin(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot change the password")
	}
	defer tx.Rollback(ctx) // no-op once committed
	q := apiCfg.DB.WithTx(tx)

	err = q.UpdateUserPassword(ctx, database.UpdateUserPasswordParams{
		ID:                   auth.User.ID,
		AuthHash:             authHash,
		KdfAlgo:              KdfAlgo_ARGON2ID13,
		KdfSalt:              params.KDFSalt,
		KdfMemKib:            KdfMem_MODERATE,
		KdfIterations:        KdfOps_MODERATE,
		EncryptedMasterKey:   params.EncryptedMasterKey,
		EncryptedMasterKeyIv: params.EncryptedMasterKeyIV,
		EncryptedPrivKey:     params.EncryptedPrivKey,
		EncryptedPrivKeyIv:   params.EncryptedPrivKeyIv,
		PubKeyX25519:         params.PubKeyX25519,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot change the password")
	}
	ended, err := q.RevokeOtherSessions(ctx, database.RevokeOtherSessionsParams{UserID: auth.User.ID, ID: auth.SessionID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot change the password")
	}
	if err := tx.Commit(ctx); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Cannot change the password")
	}
	return c.JSON(http.StatusOK, map[string]int64{"ended": ended})
}
