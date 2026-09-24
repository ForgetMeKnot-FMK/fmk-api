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
