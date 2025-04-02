package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type authController struct {
	// Add any necessary dependencies here, such as a service or repository
	authService services.AuthService
}

func NewAuthController(authService services.AuthService) http.Handler {
	// Initialize the authController with the provided authService
	c := &authController{
		authService: authService,
	}

	// Create a new router and define the routes
	r := chi.NewRouter()
	r.Post("/login", c.loginUser)
	r.Get("/logout", c.logoutUser)
	
	return r
}

func (c *authController) loginUser(w http.ResponseWriter, r *http.Request) {
	user := models.AuthRequest{}
	endpoint.ServeRequest[*models.User](
		endpoint.InternalRequest{
			W:          w,
			R:          r,
			ReqBodyObj: &user,
			EndpointLogic: func() (any, error) {
				return c.authService.AuthenticateUser(r.Context(), user.ToModel())
			},
			SuccessCode: http.StatusOK,
		},
	)
}

func (c *authController) logoutUser(w http.ResponseWriter, r *http.Request) {
	// Implement the logic to log out a user
	// You can use c.authService to call the appropriate service method
	// For example:
	// err := c.authService.LogoutUser(req)
	// if err != nil {
	// 	http.Error(res, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// res.WriteHeader(http.StatusOK)
}
