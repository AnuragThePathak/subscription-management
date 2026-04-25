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
		slog.Error("Failed to configure logger",
			logattr.Env(cf.Env),
			logattr.OtelEnabled(cf.OTel.Enabled),
			logattr.Error(err),
		)
		os.Exit(1)
	}

	slog.Info("Starting Subscription Management Service",
		logattr.Env(cf.Env),
		logattr.Port(cf.Server.Port),
	)

	// Initialize OpenTelemetry (must be after logger, before DB/Redis so future phases can trace them).
	var otelProvider *observability.Provider
	if cf.OTel.Enabled {
		otelConfig := cf.OTel
		otelConfig.Environment = cf.Env
		if otelProvider, err = observability.InitOTel(ctx, otelConfig); err != nil {
			slog.Error("Failed to initialize OpenTelemetry",
				logattr.Jaeger(otelConfig.JaegerEndpoint),
				logattr.Error(err),
			)
		}
	} else {
		slog.Warn("OpenTelemetry disabled",
			logattr.Env(cf.Env),
			logattr.OtelEnabled(cf.OTel.Enabled),
		)
	}

	// Initialize the database client
	var database *adapters.Database
	{
		dbConfig := cf.Database
		if database, err = config.DatabaseConnection(dbConfig, cf.OTel.Enabled); err != nil {
			slog.Error("Failed to initialize database client",
				logattr.Host(dbConfig.Host),
				logattr.Port(dbConfig.Port),
				logattr.Database(dbConfig.Name),
				logattr.Error(err),
			)
			os.Exit(1)
		}
		if err = database.Ping(ctx); err != nil {
			slog.Error("Failed to connect to database",
				logattr.Host(dbConfig.Host),
				logattr.Port(dbConfig.Port),
				logattr.Database(dbConfig.Name),
				logattr.Error(err),
			)
			os.Exit(1)
		}
	}

	var redis *adapters.Redis
	{
		redisConfig := cf.Redis
		if redis, err = config.RedisConnection(redisConfig, cf.OTel.Enabled); err != nil {
			slog.Error("Failed initialize Redis client",
				logattr.Host(redisConfig.Host),
				logattr.Port(redisConfig.Port),
				logattr.RedisDB(redisConfig.DB),
				logattr.Error(err),
			)
			os.Exit(1)
		}
		if err = redis.Ping(ctx); err != nil {
			slog.Error("Failed to connect to Redis",
				logattr.Host(redisConfig.Host),
				logattr.Port(redisConfig.Port),
				logattr.RedisDB(redisConfig.DB),
				logattr.Error(err),
			)
			os.Exit(1)
		}
	}

	// Initialize business dependencies
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

	// Transaction executor for running multiple operations in a single transaction
	txnExecutor := repositories.NewTxnExecutor(database.Client)

	// Initialize business metrics adapter
	var metricsPort *observability.OTelMetricsAdapter
	if cf.OTel.Enabled {
		type appMetricsState struct {
			repositories.SubscriptionRepository
		}

		metricsPort, err = observability.NewMetricsAdapter(cf.OTel,
			appMetricsState{
				subscriptionRepository,
			})
		if err != nil {
			slog.Error("Failed to initialize business metrics adapter",
				logattr.Env(cf.Env),
				logattr.OtelEnabled(cf.OTel.Enabled),
				logattr.Error(err))
		}

		if err := observability.InitQueueMetrics(
			cf.OTel.ServiceName,
			config.QueueRedisConfig(cf.Redis),
		); err != nil {
			slog.Error("Failed to initialize queue metrics",
				logattr.Env(cf.Env),
				logattr.OtelEnabled(cf.OTel.Enabled),
				logattr.Error(err))
		}
	} else {
		// Noop instruments — domain layer calls are safe no-ops.
		metricsPort = observability.NewNoOpMetricsAdapter()
		slog.Info("Business metrics adapter creation skipped",
			logattr.Env(cf.Env),
			logattr.OtelEnabled(cf.OTel.Enabled),
		)
	}

	appRateLimiterService := services.NewRateLimiterService(
		redisRateLimiter,
		config.NewRateLimit(cf.RateLimiter.App),
		"app",
	)
	jwtService := services.NewJWTService(cf.JWT, time.Now)

	subscriptionService := services.NewSubscriptionService(
		txnExecutor.WithTransaction,
		subscriptionRepository,
		billRepository,
		metricsPort,
		time.Now,
	)
	userService := services.NewUserService(userRepository, subscriptionService, time.Now)
	authService := services.NewAuthService(userService, jwtService)

	var schedulerAdapter *adapters.Scheduler
	var schedulerWorkerAdapter *adapters.QueueWorker
	{
		if slices.Contains(cf.Scheduler.EnabledForEnv, cf.Env) {
			sch := scheduler.NewSubscriptionScheduler(
				subscriptionService,
				redis.Client,
				config.QueueRedisConfig(cf.Redis),
				cf.Scheduler.Interval,
				cf.Scheduler.ReminderDays,
				cf.Scheduler.StartupDelay,
				cf.Asynq.QueueName,
				cf.Scheduler.Name,
				time.Now,
			)
			go func() {
				if startErr := sch.Start(ctx); startErr != nil && startErr != context.Canceled {
					slog.Error("Scheduler failed",
						logattr.SchedulerName(cf.Scheduler.Name),
						logattr.Queue(cf.Asynq.QueueName),
						logattr.Error(startErr),
					)
				}
			}()

			schedulerAdapter = &adapters.Scheduler{
				Scheduler: sch,
			}
		} else {
			slog.Info("Scheduler skipped",
				logattr.Env(cf.Env),
				logattr.SchedulerName(cf.Scheduler.Name),
				logattr.EnabledForEnv(cf.Scheduler.EnabledForEnv),
			)
		}

		if slices.Contains(cf.QueueWorker.EnabledForEnv, cf.Env) {
			worker := scheduler.NewQueueWorker(
				subscriptionService,
				userService,
				notifications.NewEmailSender(cf.Email),
				redis.Client,
				config.QueueRedisConfig(cf.Redis),
				cf.QueueWorker.Concurrency,
				cf.Asynq.QueueName,
				cf.QueueWorker.Name,
				time.Now,
			)
			if startErr := worker.Start(); startErr != nil && startErr != context.Canceled {
				slog.Error("Queue worker failed",
					logattr.WorkerName(cf.QueueWorker.Name),
					logattr.Queue(cf.Asynq.QueueName),
					logattr.Concurrency(cf.QueueWorker.Concurrency),
					logattr.Error(startErr))
				os.Exit(0)
			}

			schedulerWorkerAdapter = &adapters.QueueWorker{
				Worker: worker,
			}
		} else {
			slog.Info("Queue worker skipped",
				logattr.Env(cf.Env),
				logattr.WorkerName(cf.QueueWorker.Name),
				logattr.EnabledForEnv(cf.QueueWorker.EnabledForEnv),
			)
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

	slog.Info("Service ready",
		logattr.StartupTime(time.Since(startupStart)),
		logattr.Port(cf.Server.Port),
		logattr.Timeout(cf.Server.RequestTimeout),
		logattr.TLSEnabled(cf.Server.TLS.Enabled),
	)

	apiServer.StartWithGracefulShutdown(
		ctx,
		10*time.Second,
		cleanupHandlers...,
	)

	slog.Info("Service shutdown completed")
}
