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
		if cf, err = config.LoadConfig(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to load config", slog.Any("error", err))
			os.Exit(1)
		}
	}

	// Configure the default slog logger.
	if err = config.SetupLogger(ctx, cf.Env, cf.OTel.Enabled); err != nil {
		slog.ErrorContext(ctx, "Failed to configure logger", slog.Any("error", err))
		os.Exit(1)
	}

	slog.InfoContext(ctx, "Starting Subscription Management Service",
		slog.String("environment", cf.Env),
		slog.Int("port", cf.Server.Port),
	)

	// Initialize OpenTelemetry (must be after logger, before DB/Redis so future phases can trace them).
	var otelProvider *observability.Provider
	if cf.OTel.Enabled {
		cf.OTel.Environment = cf.Env
		if otelProvider, err = observability.InitOTel(ctx, cf.OTel); err != nil {
			slog.ErrorContext(ctx, "Failed to initialize OpenTelemetry", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		slog.InfoContext(ctx, "OpenTelemetry disabled")
	}

	// Initialize the database client
	var database *adapters.Database
	{
		if database, err = config.DatabaseConnection(ctx, cf.Database, cf.OTel.Enabled); err != nil {
			slog.ErrorContext(ctx, "Failed to initialize database client", slog.Any("error", err))
			os.Exit(1)
		}
		if err = database.Ping(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to connect to database", slog.Any("error", err))
			os.Exit(1)
		}
	}

	var redis *adapters.Redis
	{
		redis = config.RedisConnection(ctx, cf.Redis, cf.OTel.Enabled)
		if err = redis.Ping(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to connect to Redis", slog.Any("error", err))
			os.Exit(1)
		}
	}

	redisRateLimiter := redis_rate.NewLimiter(redis.Client)

	var userRepository repositories.UserRepository
	var subscriptionRepository repositories.SubscriptionRepository
	var billRepository repositories.BillRepository
	{
		if userRepository, err = repositories.NewUserRepository(ctx, database.DB); err != nil {
			slog.ErrorContext(ctx, "Failed to create user repository", slog.Any("error", err))
			os.Exit(1)
		}
		if subscriptionRepository, err = repositories.NewSubscriptionRepository(ctx, database.DB); err != nil {
			slog.ErrorContext(ctx, "Failed to create subscription repository", slog.Any("error", err))
			os.Exit(1)
		}
		if billRepository, err = repositories.NewBillRepository(ctx, database.DB); err != nil {
			slog.ErrorContext(ctx, "Failed to create bill repository", slog.Any("error", err))
			os.Exit(1)
		}
	}

	appRateLimiterService := services.NewRateLimiterService(redisRateLimiter, config.NewRateLimit(ctx, &cf.RateLimiter.App), "app")
	jwtService := services.NewJWTService(cf.JWT)

	var metricsPort services.SubscriptionMetrics
	if cf.OTel.Enabled {
		metricsPort, err = observability.NewMetricsAdapter(cf.OTel)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to initialize business metrics adapter", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		// Use a NoOp adapter so the domain layer requires no `if metrics != nil` checks.
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
				config.QueueRedisConfig(ctx, cf.Redis),
				cf.Scheduler.Interval,
				cf.Scheduler.ReminderDays,
				cf.Scheduler.Name,
			)
			go func() {
				if startErr := sch.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.ErrorContext(ctx, "Scheduler failed", slog.Any("error", startErr))
				}
			}()

			schedulerAdapter = &adapters.Scheduler{
				Scheduler: sch,
			}
			slog.InfoContext(ctx, "Scheduler started",
				slog.String("env", cf.Env),
				slog.Duration("interval", cf.Scheduler.Interval),
			)
		} else {
			slog.InfoContext(ctx, "Scheduler skipped", slog.String("env", cf.Env))
		}

		if slices.Contains(cf.QueueWorker.EnabledForEnv, cf.Env) {
			worker := scheduler.NewReminderWorker(
				subscriptionService,
				userService,
				notifications.NewEmailSender(cf.Email),
				redis.Client,
				config.QueueRedisConfig(ctx, cf.Redis),
				cf.QueueWorker.Concurrency,
				cf.QueueWorker.QueueName,
				cf.QueueWorker.Name,
			)
			go func() {
				if startErr := worker.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.ErrorContext(ctx, "Queue worker failed", slog.Any("error", startErr))
				}
			}()

			schedulerWorkerAdapter = &adapters.SchedulerWorker{
				Worker: worker,
			}
			slog.InfoContext(ctx, "Queue worker started",
				slog.String("env", cf.Env),
				slog.Int("concurrency", cf.QueueWorker.Concurrency),
			)
		} else {
			slog.InfoContext(ctx, "Queue worker skipped", slog.String("env", cf.Env))
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
				r.Use(middlewares.OTel(cf.OTel.ServiceName))
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

	slog.InfoContext(ctx, "Service ready", slog.Duration("startup_time", time.Since(startupStart)))

	apiServer.StartWithGracefulShutdown(
		ctx,
		10*time.Second,
		cleanupHandlers...,
	)

	slog.Info("Service shutdown completed")
}
