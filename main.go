package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/AnuragThePathak/my-go-packages/srv"
	"github.com/anuragthepathak/subscription-management/internal/adapters"
	"github.com/anuragthepathak/subscription-management/internal/api/controllers"
	"github.com/anuragthepathak/subscription-management/internal/api/middlewares"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/config"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/notifications"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/anuragthepathak/subscription-management/internal/scheduler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis_rate/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Initialize OpenTelemetry (must be after logger, before DB/Redis so future phases can trace them).
	var otelProvider *observability.Provider
	if cf.OTel.Enabled {
		otelConfig := observability.Config{
			ServiceName:    cf.OTel.ServiceName,
			Environment:    cf.Env,
			JaegerEndpoint: cf.OTel.JaegerEndpoint,
		}
		if otelProvider, err = observability.InitOTel(ctx, otelConfig); err != nil {
			slog.Error("Failed to initialize OpenTelemetry",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	} else {
		slog.Info("OpenTelemetry disabled", slog.String("component", "main"))
	}

	// Improved logging for startup
	slog.Info("Starting Subscription Management Service",
		slog.String("environment", cf.Env),
		slog.Int("port", cf.Server.Port),
	)

	// Initialize the database client
	var database *adapters.Database
	{
		if database, err = config.DatabaseConnection(cf.Database); err != nil {
			slog.Error("Failed to initialize database client",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
		if err = database.Ping(ctx); err != nil {
			slog.Error("Failed to connect to database",
				slog.String("component", "main"),
				slog.Any("error", err),
			)
			os.Exit(1)
		}
	}

	var redis *adapters.Redis
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
	jwtService := services.NewJWTService(cf.JWT)
	subscriptionService := services.NewSubscriptionService(subscriptionRepository, billRepository)
	userService := services.NewUserService(userRepository, subscriptionService)
	authService := services.NewAuthService(userService, jwtService)

	var schedulerAdapter *adapters.Scheduler
	var schedulerWorkerAdapter *adapters.SchedulerWorker
	{
		if slices.Contains(cf.Scheduler.EnabledForEnv, cf.Env) {
			sch := scheduler.NewSubscriptionScheduler(
				subscriptionService,
				redis.Client,
				config.QueueRedisConfig(cf.Redis),
				cf.Scheduler.Interval,
				cf.Scheduler.ReminderDays,
			)
			go func() {
				if startErr := sch.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.Error("Scheduler failed",
						slog.String("component", "main"),
						slog.Any("error", startErr),
					)
				}
			}()

			schedulerAdapter = &adapters.Scheduler{
				Scheduler: sch,
			}
			slog.Info("Scheduler started", slog.String("env", cf.Env))
		} else {
			slog.Info("Scheduler skipped due to environment config", slog.String("env", cf.Env))
		}

		if slices.Contains(cf.QueueWorker.EnabledForEnv, cf.Env) {
			worker := scheduler.NewReminderWorker(
				subscriptionService,
				userService,
				notifications.NewEmailSender(cf.Email),
				redis.Client,
				config.QueueRedisConfig(cf.Redis),
				cf.QueueWorker.Concurrency,
				cf.QueueWorker.QueueName,
			)
			go func() {
				if startErr := worker.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.Error("Worker failed",
						slog.String("component", "main"),
						slog.Any("error", startErr),
					)
				}
			}()

			schedulerWorkerAdapter = &adapters.SchedulerWorker{
				Worker: worker,
			}
			slog.Info("Worker started", slog.String("env", cf.Env))
		} else {
			slog.Info("Worker skipped due to environment config", slog.String("env", cf.Env))
		}
	}

	var requestHandler *endpoint.RequestHandler
	{
		validate := validator.New(validator.WithRequiredStructEnabled())
		requestHandler = endpoint.NewRequestHandler(validate)
	}

	var apiServer adapters.Server
	{
		// Setup router
		r := chi.NewRouter()
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middlewares.Timeout(cf.Server.RequestTimeout))
		r.Use(middlewares.RateLimiter(appRateLimiterService))

		// Observability: Prometheus metrics endpoint (outside auth).
		if cf.OTel.Enabled {
			r.Method(http.MethodGet, "/metrics", promhttp.Handler())
		}

		// Setup routes
		r.Mount("/api/v1/auth", controllers.NewAuthController(authService, userService, requestHandler))

		// Protected routes
		r.Group(func(r chi.Router) {
			// Apply authentication middleware
			r.Use(middlewares.Authentication(jwtService))

			// User routes with authentication
			r.Mount("/api/v1/users", controllers.NewUserController(userService, requestHandler))
			r.Mount("/api/v1/subscriptions", controllers.NewSubscriptionController(subscriptionService, requestHandler))
		})

		// Create a new server configuration
		apiserverConfig := srv.ServerConfig{
			Port:        cf.Server.Port,
			TLSEnabled:  cf.Server.TLS.Enabled,
			TLSCertPath: cf.Server.TLS.CertPath,
			TLSKeyPath:  cf.Server.TLS.KeyPath,
		}

		apiServer = srv.NewServer(r, apiserverConfig)
	}

	// Build cleanup handlers — only include non-nil components.
	var cleanupHandlers []srv.CleanupHandler
	{
		cleanupHandlers = append(cleanupHandlers, database, redis) // Always not nil
		if otelProvider != nil {
			cleanupHandlers = append(cleanupHandlers, otelProvider)
		}
		if schedulerAdapter != nil {
			cleanupHandlers = append(cleanupHandlers, schedulerAdapter)
		}
		if schedulerWorkerAdapter != nil {
			cleanupHandlers = append(cleanupHandlers, schedulerWorkerAdapter)
		}
	}

	apiServer.StartWithGracefulShutdown(
		ctx,
		10*time.Second,
		cleanupHandlers...,
	)

	slog.Info("Server shutdown completed")
}
