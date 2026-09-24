package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"time"

	crypto_rand "crypto/rand"

	"golang.org/x/crypto/nacl/box"

	"github.com/alexedwards/argon2id"
	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/medama-io/go-useragent"
)

// TODO: Tell these to the client on a GET request on the register flow
const KeyVersion = 1
const KdfAlgo_ARGON2ID13 = 2
const KdfOps_MODERATE = 3
const KdfMem_MODERATE = 268435456
const AuthHashSaltBytes = 16

func (apiCfg *apiConfig) handleUserRegistration(c *echo.Context) error {
	type parameters struct {
		Name                 string `json:"name"`
		Username             string `json:"username"`
		KDFSalt              []byte `json:"kdf_salt"`
		AuthHashRaw          []byte `json:"authHash"`
		EncryptedMasterKey   []byte `json:"encrypted_master_key"`
		EncryptedMasterKeyIV []byte `json:"encrypted_master_key_iv"`
		PubKeyX25519         []byte `json:"pub_key_X25519"`
		EncryptedPrivKey     []byte `json:"encrypted_priv_key"`
		EncryptedPrivKeyIv   []byte `json:"encrypted_priv_key_iv"`
	}

	r := c.Request()

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}

	// Todo: Validation (use `https://github.com/go-ozzo/ozzo-validation`)

	authHashSalt := make([]byte, AuthHashSaltBytes)
	_, err = crypto_rand.Read(authHashSalt)
	if err != nil {
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Something went wrong, try again later",
		)
	}

	authHash, err := argon2id.CreateHash(
		base64.StdEncoding.EncodeToString(params.AuthHashRaw),
		argon2id.DefaultParams)

	if err != nil {
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Something went wrong, try again later",
		)
	}

	userParams := database.CreateUserParams{
		Name:                 params.Name,
		Username:             params.Username,
		KdfAlgo:              KdfAlgo_ARGON2ID13,
		KdfMemKib:            KdfMem_MODERATE,
		KdfIterations:        KdfOps_MODERATE,
		KdfSalt:              params.KDFSalt,
		AuthHash:             authHash,
		EncryptedMasterKey:   params.EncryptedMasterKey,
		EncryptedMasterKeyIv: params.EncryptedMasterKeyIV,
		EncryptedPrivKey:     params.EncryptedPrivKey,
		EncryptedPrivKeyIv:   params.EncryptedPrivKeyIv,
		PubKeyX25519:         params.PubKeyX25519,
		KeyVersion:           KeyVersion,
	}

	user, err := apiCfg.DB.CreateUser(r.Context(), userParams)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Cannot create user: %v", err))
	}

	return c.JSON(http.StatusCreated, user)
}

func (apiCfg *apiConfig) handleUserPreLogin(c *echo.Context) error {

	type parameters struct {
		Username string `json:"username"`
	}

	r := c.Request()

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}

	// TODO: validate username

	user, err := apiCfg.DB.FindUserByUsername(r.Context(), params.Username)

	if err != nil {
		return echo.NewHTTPError(
			http.StatusNotFound,
			"User not found",
		)
	}

	return c.JSON(http.StatusOK, databaseUserToUser(user))
}

func (apiCfg *apiConfig) handleUserLogin(c *echo.Context) error {
	type parameters struct {
		Username    string `json:"username"`
		AuthHashRaw []byte `json:"authHash"`
	}

	r := c.Request()

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Error parsing JSON: %v", err))
	}

	// TODO: validate username & authHash

	user, err := apiCfg.DB.FindUserByUsername(r.Context(), params.Username)

	if err != nil {
		return echo.NewHTTPError(
			http.StatusNotFound,
			"User not found",
		)
	}

	match, err := argon2id.ComparePasswordAndHash(base64.StdEncoding.EncodeToString(params.AuthHashRaw), user.AuthHash)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
	}

	if !match {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	// Generate a new session
	uaParser := useragent.NewParser()
	userAgentStr := r.UserAgent()
	ua := uaParser.Parse(r.UserAgent())

	ipStr, _ := getIP(r)
	ip, err := netip.ParseAddr(ipStr)

	// NOT the DB's ID, just an ID the client will need
	sessionID := make([]byte, 32)
	if _, err := crypto_rand.Read(sessionID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
	}
	token := base64.RawURLEncoding.EncodeToString(sessionID) // Use in the client with `Bearer XXXXXXXX`
	tokenHash := sha256.Sum256([]byte(token))                // Store in DB

	var recipientPub [32]byte
	copy(recipientPub[:], user.PubKeyX25519)

	sealedSessionID, err := box.SealAnonymous(nil, sessionID, &recipientPub, crypto_rand.Reader)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
	}

	// Check if the same IP and user agent already exists, revoke/delete the old session if so
	prevSessions, err := apiCfg.DB.GetSessionsByUserID(r.Context(), user.ID)

	for _, s := range prevSessions {
		if userAgentStr != "" && s.UserAgent.String == userAgentStr && ip.String() == s.Ip.String() {
			err := apiCfg.DB.DeleteSessionByID(r.Context(), s.ID)
			if err != nil {
				log.Println("error: could not delete session with matching useragent and ip")
			}
		}
	}

	// Create the fresh session
	sessionParams := database.CreateSessionParams{
		UserID:      user.ID,
		UserAgent:   pgtype.Text{String: userAgentStr, Valid: userAgentStr != ""},
		Ip:          &ip,
		DeviceLabel: pgtype.Text{String: ua.Device().String(), Valid: ua.Device().String() != ""},
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().UTC().Add(2 * time.Hour),
			Valid: true,
		},
		TokenHash: tokenHash[:],
	}

	_, err = apiCfg.DB.CreateSession(r.Context(), sessionParams)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Something went wrong: %v", err))
	}

	// Source: /models.go: User struct
	type UserLoginResponse struct {
		ID       pgtype.UUID `json:"id"`
		Name     string      `json:"name"`
		Username string      `json:"username"`
		// AuthHash             string             `json:"auth_hash"`
		// KdfAlgo              int32              `json:"kdf_algo"`
		// KdfSalt              []byte             `json:"kdf_salt"`
		// KdfMemKib            int32              `json:"kdf_mem_kib"`
		// KdfIterations        int32              `json:"kdf_iterations"`
		// KeyVersion           int32              `json:"key_version"`
		EncryptedMasterKey   []byte `json:"encrypted_master_key"`
		EncryptedMasterKeyIv []byte `json:"encrypted_master_key_iv"`
		EncryptedPrivKey     []byte `json:"encrypted_priv_key"`
		EncryptedPrivKeyIv   []byte `json:"encrypted_priv_key_iv"`
		PubKeyX25519         []byte `json:"pub_key_x25519"`
		// CreatedAt            pgtype.Timestamptz `json:"created_at"`
		// UpdatedAt            pgtype.Timestamptz `json:"updated_at"`
		//
		SealedSessionID []byte `json:"sealed_session_id"`
	}

	return c.JSON(200, UserLoginResponse{
		ID:                   user.ID,
		Name:                 user.Name,
		Username:             user.Username,
		EncryptedMasterKey:   user.EncryptedMasterKey,
		EncryptedMasterKeyIv: user.EncryptedMasterKeyIv,
		EncryptedPrivKey:     user.EncryptedPrivKey,
		EncryptedPrivKeyIv:   user.EncryptedPrivKeyIv,
		PubKeyX25519:         user.PubKeyX25519,
		SealedSessionID:      sealedSessionID,
	})
}
