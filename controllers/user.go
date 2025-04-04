package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type userController struct {
	// Add any necessary dependencies here, such as a service or repository
	userService services.UserService
}

func NewUserController(userService services.UserService) http.Handler {
	// Initialize the userController with any necessary dependencies
	c := &userController{
		userService,
	}

	// Create a new router and define the routes
	r := chi.NewRouter()
	r.Get("/", c.getAllUsers)
	r.Get("/{id}", c.getUserByID)
	r.Put("/{id}", c.updateUser)
	r.Delete("/{id}", c.deleteUser)
	return r
}

func (c *userController) getAllUsers(w http.ResponseWriter, r *http.Request) {
	endpoint.ServeRequest(
		endpoint.InternalRequest{
			W: w,
			R: r,
			EndpointLogic: func() (any, error) {
				return endpoint.ToResponseSlice(c.userService.GetAllUsers(r.Context()))
			},
			SuccessCode: http.StatusOK,
		},
	)
}

func (c *userController) getUserByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		ReqBodyObj: nil,
		EndpointLogic: func() (any, error) {
			// Convert the internal user model to a response model using the helper.
			// If c.userService.GetUserByID returns (*models.User, error),
			// then ToResponse converts it to (*models.UserResponse, error).
			return endpoint.ToResponse(c.userService.GetUserByID(r.Context(), id))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *userController) updateUser(w http.ResponseWriter, r *http.Request) {
	// Implement the logic to update a user
	// You can use c.authService to call the appropriate service method
	// For example:
	// user, err := c.authService.UpdateUser(req)
	// if err != nil {
	// 	http.Error(res, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// res.WriteHeader(http.StatusOK)
	// json.NewEncoder(res).Encode(user)
}

func (c *userController) deleteUser(w http.ResponseWriter, r *http.Request) {
	// Implement the logic to delete a user
	// You can use c.authService to call the appropriate service method
	// For example:
	// err := c.authService.DeleteUser(req)
	// if err != nil {
	// 	http.Error(res, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// res.WriteHeader(http.StatusOK)
}
