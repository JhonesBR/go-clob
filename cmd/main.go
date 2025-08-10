package main

import (
	"log"

	"github.com/JhonesBR/go-clob/internal/api"
	"github.com/JhonesBR/go-clob/internal/db"
	"github.com/gofiber/fiber/v3"
)

func main() {
	// Initialize a new Fiber app
	app := fiber.New()

	// DB connection
	pool := db.NewConnection()
	defer pool.Close()

	// Initialize the API routes
	api.InitializeRoutes(app, pool)

	// Start the server on port 8000
	log.Fatal(app.Listen(":8000"))
}
