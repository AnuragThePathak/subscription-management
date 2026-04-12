package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/adapters"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/go-chi/chi/v5"
)

type healthController struct {
	db    *adapters.Database
	redis *adapters.Redis
}

func NewHealthController(db *adapters.Database, redis *adapters.Redis) http.Handler {
	c := &healthController{
		db:    db,
		redis: redis,
	}

	r := chi.NewRouter()
	r.Get("/healthz", c.healthz)
	r.Get("/readyz", c.readyz)
	return r
}

func (c *healthController) healthz(w http.ResponseWriter, r *http.Request) {
	endpoint.WriteAPIResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (c *healthController) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := c.db.Ping(ctx); err != nil {
		endpoint.WriteAPIResponse(
			w,
			http.StatusServiceUnavailable,
			map[string]string{"status": "unavailable", "reason": "db_ping_failed"},
		)
		return
	}

	if err := c.redis.Ping(ctx); err != nil {
		endpoint.WriteAPIResponse(
			w,
			http.StatusServiceUnavailable,
			map[string]string{"status": "unavailable", "reason": "redis_ping_failed"},
		)
		return
	}

	endpoint.WriteAPIResponse(w, http.StatusOK, map[string]string{"status": "ready"})
}
