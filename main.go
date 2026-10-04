package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ishraq10199/fmk-api/internal/database"
)

type apiConfig struct {
	DB   *database.Queries
	Pool *pgxpool.Pool // for transactions (see revision.go)
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

	apiCfg := apiConfig{DB: database.New(pool), Pool: pool}

	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			// "http://127.0.0.1:5500",
			// "http://localhost:5500",
			// "http://127.0.0.1:5173",
			// "http://localhost:5173",
			"*",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
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
