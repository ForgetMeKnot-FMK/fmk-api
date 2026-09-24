package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ishraq10199/fmk-api/internal/database"
)

type apiConfig struct {
	DB *database.Queries
}

func main() {
	log.Println("FMK API")

	godotenv.Load()

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, os.Getenv("DB_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	log.Println("DB Connected!")

	apiCfg := apiConfig{DB: database.New(conn)}

	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"http://127.0.0.1:5500",
			"http://localhost:5500",
			"http://127.0.0.1:5173",
			"http://localhost:5173",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.POST("/api/v1/register", apiCfg.handleUserRegistration)
	e.POST("/api/v1/prelogin", apiCfg.handleUserPreLogin)
	e.POST("/api/v1/login", apiCfg.handleUserLogin)
	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Hello from Echo!"})
	})

	if err := e.Start(":8000"); err != nil {
		e.Logger.Error("failed to start the server - error:", err)
	}
}
