package main

import (
	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/medama-io/go-useragent"
)

// This is made to change the json key names as we need, to send to the client
// Source: `/internal/database/models.go`: User struct
type User struct {
	ID                   pgtype.UUID        `json:"id"`
	Name                 string             `json:"name"`
	Username             string             `json:"username"`
	AuthHash             string             `json:"auth_hash"`
	KdfAlgo              int32              `json:"kdf_algo"`
	KdfSalt              []byte             `json:"kdf_salt"`
	KdfMemKib            int32              `json:"kdf_mem_kib"`
	KdfIterations        int32              `json:"kdf_iterations"`
	KeyVersion           int32              `json:"key_version"`
	EncryptedMasterKey   []byte             `json:"encrypted_master_key"`
	EncryptedMasterKeyIv []byte             `json:"encrypted_master_key_iv"`
	EncryptedPrivKey     []byte             `json:"encrypted_priv_key"`
	EncryptedPrivKeyIv   []byte             `json:"encrypted_priv_key_iv"`
	PubKeyX25519         []byte             `json:"pub_key_x25519"`
	CreatedAt            pgtype.Timestamptz `json:"created_at"`
	UpdatedAt            pgtype.Timestamptz `json:"updated_at"`
}

// Should follow the above
// What POST /prelogin sends: only the settings the client needs to derive the
// password's keys. It is answered before any login, so it must not include
// anything a guess could be checked against (the stored hash, the encrypted keys).
type PreloginParams struct {
	KdfAlgo       int32  `json:"kdf_algo"`
	KdfSalt       []byte `json:"kdf_salt"`
	KdfMemKib     int32  `json:"kdf_mem_kib"`
	KdfIterations int32  `json:"kdf_iterations"`
}

func databaseUserToPreloginParams(dbUser database.User) PreloginParams {
	return PreloginParams{
		KdfAlgo:       dbUser.KdfAlgo,
		KdfSalt:       dbUser.KdfSalt,
		KdfMemKib:     dbUser.KdfMemKib,
		KdfIterations: dbUser.KdfIterations,
	}
}

func databaseUserToUser(dbUser database.User) User {
	return User{
		ID:                   dbUser.ID,
		Name:                 dbUser.Name,
		Username:             dbUser.Username,
		AuthHash:             dbUser.AuthHash,
		KdfAlgo:              dbUser.KdfAlgo,
		KdfSalt:              dbUser.KdfSalt,
		KdfMemKib:            dbUser.KdfMemKib,
		KdfIterations:        dbUser.KdfIterations,
		KeyVersion:           dbUser.KeyVersion,
		EncryptedMasterKey:   dbUser.EncryptedMasterKey,
		EncryptedMasterKeyIv: dbUser.EncryptedMasterKeyIv,
		EncryptedPrivKey:     dbUser.EncryptedPrivKey,
		EncryptedPrivKeyIv:   dbUser.EncryptedPrivKeyIv,
		PubKeyX25519:         dbUser.PubKeyX25519,
		CreatedAt:            dbUser.CreatedAt,
		UpdatedAt:            dbUser.UpdatedAt,
	}
}

type Knot struct {
	ID                 pgtype.UUID        `json:"id"`
	OwnerID            pgtype.UUID        `json:"owner_id"`
	EncryptedContent   []byte             `json:"encrypted_content"`
	EncryptedContentIv []byte             `json:"encrypted_content_iv"`
	WrappedKnotKey     []byte             `json:"wrapped_knot_key"`
	WrappedKnotKeyIv   []byte             `json:"wrapped_knot_key_iv"`
	EncryptedTitle     []byte             `json:"encrypted_title"`
	EncryptedTitleIv   []byte             `json:"encrypted_title_iv"`
	KeyVersion         int32              `json:"key_version"`
	Revision           int64              `json:"revision"`
	CreatedAt          pgtype.Timestamptz `json:"created_at"`
	UpdatedAt          pgtype.Timestamptz `json:"updated_at"`
}

func databaseKnotToKnot(dbKnot database.Knot) Knot {
	return Knot{
		ID:                 dbKnot.ID,
		OwnerID:            dbKnot.OwnerID,
		EncryptedContent:   dbKnot.EncryptedContent,
		EncryptedContentIv: dbKnot.EncryptedContentIv,
		WrappedKnotKey:     dbKnot.WrappedKnotKey,
		WrappedKnotKeyIv:   dbKnot.WrappedKnotKeyIv,
		EncryptedTitle:     dbKnot.EncryptedTitle,
		EncryptedTitleIv:   dbKnot.EncryptedTitleIv,
		KeyVersion:         dbKnot.KeyVersion,
		Revision:           dbKnot.Revision,
		CreatedAt:          dbKnot.CreatedAt,
		UpdatedAt:          dbKnot.UpdatedAt,
	}
}

type KnotAttribute struct {
	KnotID     pgtype.UUID        `json:"knot_id"`
	OwnerID    pgtype.UUID        `json:"owner_id"`
	Data       []byte             `json:"data"`
	DataIv     []byte             `json:"data_iv"`
	KeyVersion int32              `json:"key_version"`
	Revision   int64              `json:"revision"`
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"`
}

func databaseKnotAttributeToKnotAttribute(dbKnotAttribute database.KnotAttribute) KnotAttribute {
	return KnotAttribute{
		KnotID:     dbKnotAttribute.KnotID,
		OwnerID:    dbKnotAttribute.OwnerID,
		Data:       dbKnotAttribute.Data,
		DataIv:     dbKnotAttribute.DataIv,
		KeyVersion: dbKnotAttribute.KeyVersion,
		Revision:   dbKnotAttribute.Revision,
		UpdatedAt:  dbKnotAttribute.UpdatedAt,
	}
}

type UserAttribute struct {
	UserID     pgtype.UUID        `json:"user_id"`
	Data       []byte             `json:"data"`
	DataIv     []byte             `json:"data_iv"`
	KeyVersion int32              `json:"key_version"`
	Revision   int64              `json:"revision"`
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"`
}

func databaseUserAttributeToUserAttribute(dbUserAttribute database.UserAttribute) UserAttribute {
	return UserAttribute{
		UserID:     dbUserAttribute.UserID,
		Data:       dbUserAttribute.Data,
		DataIv:     dbUserAttribute.DataIv,
		KeyVersion: dbUserAttribute.KeyVersion,
		Revision:   dbUserAttribute.Revision,
		UpdatedAt:  dbUserAttribute.UpdatedAt,
	}
}

// Answers to writes: only what the client doesn't already have. It sent the
// encrypted data itself, so echoing it back would only cost bandwidth.

// POST and PUT /knot
type KnotWriteResult struct {
	ID        pgtype.UUID        `json:"id"`
	Revision  int64              `json:"revision"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

func databaseKnotToKnotWriteResult(dbKnot database.Knot) KnotWriteResult {
	return KnotWriteResult{
		ID:        dbKnot.ID,
		Revision:  dbKnot.Revision,
		CreatedAt: dbKnot.CreatedAt,
		UpdatedAt: dbKnot.UpdatedAt,
	}
}

// POST and PUT /knot/:id/attr
type KnotAttributeWriteResult struct {
	KnotID    pgtype.UUID        `json:"knot_id"`
	Revision  int64              `json:"revision"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

func databaseKnotAttributeToWriteResult(dbAttr database.KnotAttribute) KnotAttributeWriteResult {
	return KnotAttributeWriteResult{
		KnotID:    dbAttr.KnotID,
		Revision:  dbAttr.Revision,
		UpdatedAt: dbAttr.UpdatedAt,
	}
}

// POST and PUT /user/attr (a 409 still sends the whole current row: the client merges into it)
type UserAttributeWriteResult struct {
	Revision  int64              `json:"revision"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

func databaseUserAttributeToWriteResult(dbAttr database.UserAttribute) UserAttributeWriteResult {
	return UserAttributeWriteResult{Revision: dbAttr.Revision, UpdatedAt: dbAttr.UpdatedAt}
}

// POST /register: the new user's id (the client has everything else)
type RegisterResult struct {
	ID pgtype.UUID `json:"id"`
}

// What GET /user sends: who is logged in. No keys, no hash
type Profile struct {
	ID        pgtype.UUID        `json:"id"`
	Name      string             `json:"name"`
	Username  string             `json:"username"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func databaseUserToProfile(dbUser database.User) Profile {
	return Profile{ID: dbUser.ID, Name: dbUser.Name, Username: dbUser.Username, CreatedAt: dbUser.CreatedAt}
}

// What GET /sessions sends per session. Never the token hash
type Session struct {
	ID          pgtype.UUID        `json:"id"`
	DeviceLabel *string            `json:"device_label"`
	UserAgent   *string            `json:"user_agent"`
	Browser     string             `json:"browser"` // from the user agent, e.g. "Firefox" ("" if unknown)
	OS          string             `json:"os"`      // e.g. "Linux"
	IP          *string            `json:"ip"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
	LastUsedAt  pgtype.Timestamptz `json:"last_used_at"`
	ExpiresAt   pgtype.Timestamptz `json:"expires_at"`
	Current     bool               `json:"current"` // the session making the request
}

// Parses user agents for the session list (building it takes a moment: once)
var sessionUAParser = useragent.NewParser()

func databaseSessionToSession(s database.Session, currentID pgtype.UUID) Session {
	text := func(t pgtype.Text) *string {
		if !t.Valid {
			return nil
		}
		return &t.String
	}
	var ip *string
	if s.Ip != nil {
		v := s.Ip.String()
		ip = &v
	}
	ua := sessionUAParser.Parse(s.UserAgent.String)
	return Session{
		ID:          s.ID,
		DeviceLabel: text(s.DeviceLabel),
		UserAgent:   text(s.UserAgent),
		Browser:     string(ua.Browser()),
		OS:          string(ua.OS()),
		IP:          ip,
		CreatedAt:   s.CreatedAt,
		LastUsedAt:  s.LastUsedAt,
		ExpiresAt:   s.ExpiresAt,
		Current:     s.ID == currentID,
	}
}
