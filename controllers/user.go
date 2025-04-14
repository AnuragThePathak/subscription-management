package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/middlewares"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type userController struct {
	userService services.UserService
}

func NewUserController(userService services.UserService) http.Handler {
	c := &userController{userService}

	r := chi.NewRouter()
	r.Get("/", c.getAllUsers)
	r.Get("/{id}", c.getUserByID)
	r.Put("/{id}", c.updateUser)
	r.Delete("/{id}", c.deleteUser)
	return r
}

func (c *userController) getAllUsers(w http.ResponseWriter, r *http.Request) {
	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponseSlice(c.userService.GetAllUsers(r.Context()))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *userController) getUserByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claimedUserID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.userService.GetUserByID(r.Context(), id, claimedUserID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *userController) updateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claimedUserID, _ := middlewares.GetUserID(r.Context())
	updateReq := models.UserUpdateRequest{}

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		ReqBodyObj: &updateReq,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.userService.UpdateUser(r.Context(), id, &updateReq, claimedUserID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *userController) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claimedUserID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return nil, c.userService.DeleteUser(r.Context(), id, claimedUserID)
		},
		SuccessCode: http.StatusNoContent,
	})
}
