package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/go-chi/chi/v5"
)

type userController struct {
	userService    services.UserServiceExternal
	requestHandler *endpoint.RequestHandler
}

func NewUserController(userService services.UserServiceExternal, requestHandler *endpoint.RequestHandler) http.Handler {
	c := &userController{userService, requestHandler}

	r := chi.NewRouter()
	r.Get("/", c.getAllUsers)
	r.Get("/{id}", c.getUserByID)
	r.Put("/{id}", c.updateUser)
	r.Delete("/{id}", c.deleteUser)
	return r
}

func (c *userController) getAllUsers(w http.ResponseWriter, r *http.Request) {
	c.requestHandler.ServeRequest(endpoint.InternalRequest{
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
	claimedUserID, _ := lib.GetUserID(r.Context())

	c.requestHandler.ServeRequest(endpoint.InternalRequest{
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
	claimedUserID, _ := lib.GetUserID(r.Context())
	updateReq := models.UserUpdateRequest{}

	c.requestHandler.ServeRequest(endpoint.InternalRequest{
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
	claimedUserID, _ := lib.GetUserID(r.Context())

	c.requestHandler.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return nil, c.userService.DeleteUser(r.Context(), id, claimedUserID)
		},
		SuccessCode: http.StatusNoContent,
	})
}
