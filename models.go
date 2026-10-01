package main

import (
	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
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
	UpdatedAt  pgtype.Timestamptz `json:"updated_at"`
}

func databaseKnotAttributeToKnotAttribute(dbKnotAttribute database.KnotAttribute) KnotAttribute {
	return KnotAttribute{
		KnotID:     dbKnotAttribute.KnotID,
		OwnerID:    dbKnotAttribute.OwnerID,
		Data:       dbKnotAttribute.Data,
		DataIv:     dbKnotAttribute.DataIv,
		KeyVersion: dbKnotAttribute.KeyVersion,
		UpdatedAt:  dbKnotAttribute.UpdatedAt,
	}
}
