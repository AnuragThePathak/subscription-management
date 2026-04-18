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
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/config"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
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
	startupStart := time.Now()
	var err error
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	var cf *config.Config
	{
		if cf, err = config.LoadConfig(); err != nil {
			slog.Error("Failed to load config", logattr.Error(err))
			os.Exit(1)
		}
	}

	// Configure the default slog logger.
	if err = config.SetupLogger(cf.Env, cf.OTel.Enabled); err != nil {
		slog.Error("Failed to configure logger", logattr.Error(err))
		os.Exit(1)
	}

	slog.Info("Starting Subscription Management Service",
		logattr.Env(cf.Env),
		logattr.Port(cf.Server.Port),
	)

	// Initialize OpenTelemetry (must be after logger, before DB/Redis so future phases can trace them).
	var otelProvider *observability.Provider
	if cf.OTel.Enabled {
		cf.OTel.Environment = cf.Env
		if otelProvider, err = observability.InitOTel(ctx, cf.OTel); err != nil {
			slog.Error("Failed to initialize OpenTelemetry", logattr.Error(err))
			os.Exit(1)
		}
	} else {
		slog.Info("OpenTelemetry disabled")
	}

	// Initialize the database client
	var database *adapters.Database
	{
		if database, err = config.DatabaseConnection(cf.Database, cf.OTel.Enabled); err != nil {
			slog.Error("Failed to initialize database client", logattr.Error(err))
			os.Exit(1)
		}
		if err = database.Ping(ctx); err != nil {
			slog.Error("Failed to connect to database", logattr.Error(err))
			os.Exit(1)
		}
	}

	var redis *adapters.Redis
	{
		redis = config.RedisConnection(cf.Redis, cf.OTel.Enabled)
		if err = redis.Ping(ctx); err != nil {
			slog.Error("Failed to connect to Redis", logattr.Error(err))
			os.Exit(1)
		}
	}

	redisRateLimiter := redis_rate.NewLimiter(redis.Client)

	var userRepository repositories.UserRepository
	var subscriptionRepository repositories.SubscriptionRepository
	var billRepository repositories.BillRepository
	{
		if userRepository, err = repositories.NewUserRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create user repository", logattr.Error(err))
			os.Exit(1)
		}
		if subscriptionRepository, err = repositories.NewSubscriptionRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create subscription repository", logattr.Error(err))
			os.Exit(1)
		}
		if billRepository, err = repositories.NewBillRepository(ctx, database.DB); err != nil {
			slog.Error("Failed to create bill repository", logattr.Error(err))
			os.Exit(1)
		}
	}

	appRateLimiterService := services.NewRateLimiterService(
		redisRateLimiter,
		config.NewRateLimit(&cf.RateLimiter.App),
		"app",
	)
	jwtService := services.NewJWTService(cf.JWT)

	var metricsPort *observability.OTelMetricsAdapter
	if cf.OTel.Enabled {
		metricsPort, err = observability.NewMetricsAdapter(cf.OTel)
		if err != nil {
			slog.Error("Failed to initialize business metrics adapter", logattr.Error(err))
			os.Exit(1)
		}
	} else {
		// Noop instruments — domain layer calls are safe no-ops.
		metricsPort = observability.NewNoOpMetricsAdapter()
	}

	subscriptionService := services.NewSubscriptionService(subscriptionRepository, billRepository, metricsPort)
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
				cf.Scheduler.StartupDelay,
				cf.QueueWorker.QueueName,
				cf.Scheduler.Name,
			)
			go func() {
				if startErr := sch.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.Error("Scheduler failed", logattr.Error(startErr))
				}
			}()

			schedulerAdapter = &adapters.Scheduler{
				Scheduler: sch,
			}
			slog.Info("Scheduler started",
				logattr.Env(cf.Env),
				logattr.Interval(cf.Scheduler.Interval),
			)
		} else {
			slog.Info("Scheduler skipped", logattr.Env(cf.Env))
		}

		if slices.Contains(cf.QueueWorker.EnabledForEnv, cf.Env) {
			worker := scheduler.NewQueueWorker(
				subscriptionService,
				userService,
				notifications.NewEmailSender(cf.Email),
				redis.Client,
				config.QueueRedisConfig(cf.Redis),
				cf.QueueWorker.Concurrency,
				cf.QueueWorker.QueueName,
				cf.QueueWorker.Name,
			)
			go func() {
				if startErr := worker.Start(); startErr != nil && startErr != context.Canceled {
					slog.Error("Queue worker failed", logattr.Error(startErr))
				}
			}()

			schedulerWorkerAdapter = &adapters.SchedulerWorker{
				Worker: worker,
			}
			slog.Info("Queue worker started",
				logattr.Env(cf.Env),
				logattr.Concurrency(cf.QueueWorker.Concurrency),
			)
		} else {
			slog.Info("Queue worker skipped", logattr.Env(cf.Env))
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

		// Observability: Prometheus metrics endpoint — always exposed so
		// infrastructure tooling (healthchecks, Prometheus) can scrape it
		// regardless of whether OTel tracing is enabled.
		r.Method(http.MethodGet, "/metrics", promhttp.Handler())

		// Health Checks
		r.Mount("/", controllers.NewHealthController(database, redis))

		// Service Specific API Group
		r.Group(func(r chi.Router) {
			// Observability: OTel middleware first to capture the full request lifecycle.
			// Ensures trace_id is injected into r.Context() for subsequent middlewares (like Logger).
			if cf.OTel.Enabled {
				r.Use(middlewares.OTel())
			}
			r.Use(middleware.Recoverer)
			r.Use(middleware.Logger)
			r.Use(middlewares.Timeout(cf.Server.RequestTimeout))
			r.Use(middlewares.RateLimiter(appRateLimiterService))

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

	slog.Info("Service ready", logattr.StartupTime(time.Since(startupStart)))

	apiServer.StartWithGracefulShutdown(
		ctx,
		10*time.Second,
		cleanupHandlers...,
	)

	slog.Info("Service shutdown completed")
}
