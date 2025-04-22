package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/lib"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type subscriptionController struct {
	subscriptionService services.SubscriptionServiceExternal
}

func NewSubscriptionController(subscriptionService services.SubscriptionServiceExternal) http.Handler {
	c := &subscriptionController{
		subscriptionService,
	}

	r := chi.NewRouter()
	r.Post("/", c.createSubscription)
	r.Get("/", c.getAllSubscriptions)
	r.Get("/user/{id}", c.getSubscriptionsByUserID)
	r.Get("/{id}", c.getSubscriptionByID)
	r.Put("/{id}/cancel", c.cancelSubscription)
	r.Delete("/{id}", c.deleteSubscription)

	return r
}

func (c *subscriptionController) createSubscription(w http.ResponseWriter, r *http.Request) {
	subscription := models.SubscriptionRequest{}
	userID, _ := lib.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		ReqBodyObj: &subscription,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.CreateSubscription(r.Context(), subscription.ToModel(), userID))
		},
		SuccessCode: http.StatusCreated,
	})
}

func (c *subscriptionController) getAllSubscriptions(w http.ResponseWriter, r *http.Request) {
	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponseSlice(c.subscriptionService.GetAllSubscriptions(r.Context()))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) getSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	userID, _ := lib.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.GetSubscriptionByID(r.Context(), subscriptionID, userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	userID, _ := lib.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return nil, c.subscriptionService.DeleteSubscription(r.Context(), subscriptionID, userID)
		},
		SuccessCode: http.StatusNoContent,
	})
}

func (c *subscriptionController) getSubscriptionsByUserID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, _ := lib.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponseSlice(c.subscriptionService.GetSubscriptionsByUserID(r.Context(), id, userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	userID, _ := lib.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.CancelSubscription(r.Context(), subscriptionID, userID))
		},
		SuccessCode: http.StatusOK,
	})
}
