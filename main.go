package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ishraq10199/fmk-api/internal/database"
)

type apiConfig struct {
	DB    *database.Queries
	Pool  *pgxpool.Pool // for transactions (see revision.go)
	Blobs *blobStore    // encrypted blobs on disk (see blob_store.go)
	Quota blobQuota     // per-knot blob quota (see handler_blob.go)
}

// A whole number from the environment, or `fallback` when unset or not a number
func envInt(name string, fallback int64) int64 {
	n, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func main() {
	log.Println("FMK API")

	godotenv.Load()

	ctx := context.Background()

	// A pool lets concurrent requests each use their own connection
	pool, err := pgxpool.New(ctx, os.Getenv("DB_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("DB Connected!")

	// Encrypted blobs (images): BLOB_DIR, default ./blobs
	blobDir := os.Getenv("BLOB_DIR")
	if blobDir == "" {
		blobDir = "blobs"
	}
	blobs, err := newBlobStore(blobDir)
	if err != nil {
		log.Fatal(err)
	}

	apiCfg := apiConfig{DB: database.New(pool), Pool: pool, Blobs: blobs, Quota: blobQuota{
		// 100 images, and the thumbnail of their cover
		Count: envInt("KNOT_BLOB_MAX_COUNT", 101),
		Bytes: envInt("KNOT_BLOB_QUOTA_MB", 25) << 20,
	}}

	// Unused blobs are deleted after BLOB_UNREFERENCED_DAYS (default 30): checked at start, then daily
	keepDays, err := strconv.Atoi(os.Getenv("BLOB_UNREFERENCED_DAYS"))
	if err != nil {
		keepDays = defaultUnreferencedDays
	}
	go func() {
		for {
			apiCfg.cleanBlobs(ctx, time.Duration(keepDays)*24*time.Hour)
			time.Sleep(24 * time.Hour)
		}
	}()

	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			// "http://127.0.0.1:5500",
			// "http://localhost:5500",
			// "http://127.0.0.1:5173",
			// "http://localhost:5173",
			"*",
		},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Blob-Key", "X-Blob-Key-Iv", "X-Blob-Sha256", "X-Blob-Tag"},

		AllowCredentials: true,
		UnsafeAllowOriginFunc: func(c *echo.Context, origin string) (string, bool, error) {
			return origin, true, nil
		},
		MaxAge: 300,
	}))

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.POST("/api/v1/register", apiCfg.handleUserRegistration)
	e.POST("/api/v1/prelogin", apiCfg.handleUserPreLogin)
	e.POST("/api/v1/login", apiCfg.handleUserLogin)

	e.GET("/api/v1/knot/attr", apiCfg.handleKnotAttributesList)
	e.GET("/api/v1/knot/trash", apiCfg.handleTrashList)
	e.POST("/api/v1/knot/:id/restore", apiCfg.handleKnotRestore)
	e.GET("/api/v1/knot/:id/attr", apiCfg.handleKnotAttributesGet)

	e.POST("/api/v1/knot", apiCfg.handleKnotCreation)

	e.DELETE("/api/v1/knot/:id", apiCfg.handleKnotDelete)
	e.POST("/api/v1/knot/:id/attr", apiCfg.handleKnotAttributesCreation)
	e.PUT("/api/v1/knot/:id/attr", apiCfg.handleKnotAttributesUpdate)
	e.PUT("/api/v1/knot/:id", apiCfg.handleKnotUpdate)
	e.GET("/api/v1/knot/:id", apiCfg.handleKnotGet)

	e.GET("/api/v1/user/attr", apiCfg.handleUserAttributesGet)
	e.POST("/api/v1/user/attr", apiCfg.handleUserAttributesCreation)
	e.PUT("/api/v1/user/attr", apiCfg.handleUserAttributesUpdate)

	e.GET("/api/v1/sync", apiCfg.handleSync)

	e.GET("/api/v1/blob", apiCfg.handleBlobFind)
	e.GET("/api/v1/blob/quota", apiCfg.handleBlobQuota)
	e.PUT("/api/v1/blob/:id", apiCfg.handleBlobUpload)
	e.GET("/api/v1/blob/:id", apiCfg.handleBlobGet)
	e.GET("/api/v1/blob/:id/key", apiCfg.handleBlobKey)

	e.GET("/api/v1/user", apiCfg.handleProfileGet)
	e.PUT("/api/v1/user/password", apiCfg.handlePasswordChange)
	e.GET("/api/v1/sessions", apiCfg.handleSessionsList)
	e.DELETE("/api/v1/sessions", apiCfg.handleOtherSessionsRevoke)
	e.DELETE("/api/v1/sessions/:id", apiCfg.handleSessionRevoke)
	e.POST("/api/v1/logout", apiCfg.handleLogout)

	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Hello from Echo!"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	if err := e.Start(":" + port); err != nil {
		e.Logger.Error("failed to start the server", "error", err)
	}
}
