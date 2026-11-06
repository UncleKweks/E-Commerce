package main

import (
	"github.com/UncleKweks/E-Commerce/internal/config"
	"github.com/UncleKweks/E-Commerce/internal/database"
	"github.com/UncleKweks/E-Commerce/internal/logger"
	"github.com/gin-gonic/gin"
)

func main() {

	log := logger.New()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	mainDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get database connection")
	}
	defer func() {
		if err := mainDB.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		}
	}()
	gin.SetMode(cfg.Server.GinMode)
	log.Info().Msg("Database connection established successfully")

	// Continue with the rest of your application logic here
}
