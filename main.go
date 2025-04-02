package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/AnuragThePathak/my-go-packages/srv"
	"github.com/anuragthepathak/subscription-management/controllers"
	"github.com/anuragthepathak/subscription-management/repositories"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/anuragthepathak/subscription-management/wrappers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	var err error

	// Configure the default slog logger
	setupLogger()

	// Connect to the database
	var database *wrappers.Database
	{
		if database, err = databaseConnection(); err != nil {
			slog.Error("Failed to connect to database",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}	
	}

	var userRepository repositories.UserRepository
	{
		if userRepository, err = repositories.NewUserRepository(database.DB); err != nil {
			slog.Error("Failed to create user repository",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	}

	userService := services.NewUserService(userRepository)
	authService := services.NewAuthService(userRepository)

	var apiServer wrappers.Server
	{
		// Setup router
		r := chi.NewRouter()
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		// Setup routes
		r.Mount("/api/v1/auth", controllers.NewAuthController(authService))
		r.Mount("/api/v1/users", controllers.NewUserController(userService))

		// Create a new server configuration
		apiserverConfig, err := serverConfig()
		if err != nil {
			slog.Error("Failed to load server configuration",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}

		apiServer = srv.NewServer(r, *apiserverConfig)
	}

	apiServer.StartWithGracefulShutdown(
		context.Background(),
		10*time.Second,
		database,
	)
}
