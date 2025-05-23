package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/go-chi/chi/v5"
)

type authController struct {
	authService services.AuthService
	userService services.UserServiceExternal
}

// NewAuthController initializes the authentication controller with routes.
func NewAuthController(authService services.AuthService, userService services.UserServiceExternal) http.Handler {
	c := &authController{
		authService,
		userService,
	}

	r := chi.NewRouter()
	r.Post("/login", c.login)
	r.Post("/refresh", c.refreshToken)
	r.Post("/register", c.createUser)

	return r
}

// createUser handles user registration.
func (c *authController) createUser(w http.ResponseWriter, r *http.Request) {
	user := models.UserRequest{}

	endpoint.ServeRequest(
		endpoint.InternalRequest{
			W:          w,
			R:          r,
			ReqBodyObj: &user,
			EndpointLogic: func() (any, error) {
				return endpoint.ToResponse(c.userService.CreateUser(r.Context(), user.ToModel()))
			},
			SuccessCode: http.StatusCreated,
		},
	)
}

// login handles user login and token generation.
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

// refreshToken handles token refresh requests.
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
