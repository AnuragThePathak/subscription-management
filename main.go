package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AnuragThePathak/my-go-packages/srv"
	"github.com/anuragthepathak/subscription-management/config"
	"github.com/anuragthepathak/subscription-management/controllers"
	"github.com/anuragthepathak/subscription-management/email"
	"github.com/anuragthepathak/subscription-management/middlewares"
	"github.com/anuragthepathak/subscription-management/queue"
	"github.com/anuragthepathak/subscription-management/repositories"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/anuragthepathak/subscription-management/wrappers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis_rate/v10"
)

func main() {
	var err error
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	var cf *config.Config
	{
		if cf, err = config.LoadConfig(); err != nil {
			slog.Error("Failed to load config",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	}

	// Configure the default slog logger
	config.SetupLogger(cf.Env)

	// Improved logging for startup
	slog.Info("Starting Subscription Management Service",
		slog.String("environment", cf.Env),
		slog.Int("port", cf.Server.Port),
	)

	// Connect to the database
	var database *wrappers.Database
	{
		if database, err = config.DatabaseConnection(cf.Database); err != nil {
			slog.Error("Failed to connect to database",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	}

	var redis *wrappers.Redis
	{
		redis = config.RedisConnection(cf.Redis)
		if err = redis.Ping(ctx); err != nil {
			slog.Error("Failed to connect to Redis",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}

		// _ = redis.Client.FlushDB(ctx).Err()
	}

	redisRateLimiter := redis_rate.NewLimiter(redis.Client)

	var userRepository repositories.UserRepository
	var subscriptionRepository repositories.SubscriptionRepository
	var billRepository repositories.BillRepository
	{
		if userRepository, err = repositories.NewUserRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create user repository",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}

		if subscriptionRepository, err = repositories.NewSubscriptionRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create subscription repository",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}

		if billRepository, err = repositories.NewBillRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create bill repository",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	}
	
	appRateLimiterService := services.NewRateLimiterService(redisRateLimiter, config.NewRateLimit(&cf.RateLimiter.App), "app")
	userService := services.NewUserService(userRepository, subscriptionRepository)
	jwtService := services.NewJWTService(cf.JWT)
	authService := services.NewAuthService(userRepository, jwtService)
	subscriptionService := services.NewSubscriptionService(subscriptionRepository, billRepository)
	
	var scheduler *wrappers.Scheduler
	var queueWorker *wrappers.QueueWorker
	{
		sch := queue.NewSubscriptionScheduler(
			subscriptionService,
			redis.Client,
			config.QueueRedisConfig(cf.Redis),
			cf.Scheduler.Interval,
			cf.Scheduler.ReminderDays,
		)
		go func() {
			if err = sch.Start(ctx); err != nil && err != context.Canceled {
				slog.Error("Scheduler failed",
					slog.String("component", "main"),
					slog.Any("error", err),
				)
			}
		}()

		scheduler = &wrappers.Scheduler{
			Scheduler: sch,
		}

		worker := queue.NewReminderWorker(
			subscriptionService,
			userService,
			email.NewEmailSender(cf.Email),
			redis.Client,
			config.QueueRedisConfig(cf.Redis),
			cf.QueueWorker.Concurrency,
		)
		go func() {
			if err = worker.Start(ctx); err != nil && err != context.Canceled {
				slog.Error("Worker failed",
					slog.String("component", "main"),
					slog.Any("error", err),
				)
			}
		}()

		queueWorker = &wrappers.QueueWorker{
			Worker: worker,
		}
	}
	
	var apiServer wrappers.Server
	{
		// Setup router
		r := chi.NewRouter()
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middlewares.RateLimiter(appRateLimiterService))

		// Setup routes
		r.Mount("/api/v1/auth", controllers.NewAuthController(authService, userService))

		// Protected routes
		r.Group(func(r chi.Router) {
			// Apply authentication middleware
			r.Use(middlewares.Authentication(jwtService))

			// User routes with authentication
			r.Mount("/api/v1/users", controllers.NewUserController(userService))
			r.Mount("/api/v1/subscriptions", controllers.NewSubscriptionController(subscriptionService))
		})

		// Create a new server configuration
		apiserverConfig := srv.ServerConfig{
			Port:        cf.Server.Port,
			TLSEnabled:  cf.Server.TLS.Enabled,
			TLSCertPath: cf.Server.TLS.CertPath,
			TLSKeyPath:  cf.Server.TLS.KeyPath,
		}
		if err != nil {
			slog.Error("Failed to load server configuration",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}

		apiServer = srv.NewServer(r, apiserverConfig)
	}

	apiServer.StartWithGracefulShutdown(
		ctx,
		10*time.Second,
		database,
		redis,
		scheduler,
		queueWorker,
	)

	slog.Info("Server shutdown completed")
}
