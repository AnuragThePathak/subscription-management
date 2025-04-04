// controllers/auth_controller.go
package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type authController struct {
	authService services.AuthService
}

// NewAuthController creates a new auth controller
func NewAuthController(authService services.AuthService) http.Handler {
	c := &authController{
		authService,
	}

	r := chi.NewRouter()
	r.Post("/login", c.login)
	r.Post("/refresh", c.refreshToken)

	return r
}

func (c *authController) login(w http.ResponseWriter, r *http.Request) {
	loginReq := models.LoginRequest{}

	endpoint.ServeRequest(
		endpoint.InternalRequest{
			W:          w,
			R:          r,
			ReqBodyObj: &loginReq,
			EndpointLogic: func() (any, error) {
				return c.authService.Login(r.Context(), loginReq)
			},
			SuccessCode: http.StatusOK,
		},
	)
}

func (c *authController) refreshToken(w http.ResponseWriter, r *http.Request) {
	type refreshRequest struct {
		RefreshToken string `json:"refreshToken" validate:"required"`
	}

	req := refreshRequest{}

	endpoint.ServeRequest(
		endpoint.InternalRequest{
			W:          w,
			R:          r,
			ReqBodyObj: &req,
			EndpointLogic: func() (any, error) {
				return c.authService.RefreshToken(r.Context(), req.RefreshToken)
			},
			SuccessCode: http.StatusOK,
		},
	)
}