package controllers

import (
	"net/http"

	"github.com/anuragthepathak/subscription-management/endpoint"
	"github.com/anuragthepathak/subscription-management/middlewares"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/services"
	"github.com/go-chi/chi/v5"
)

type subscriptionController struct {
	subscriptionService services.SubscriptionService
}

func NewSubscriptionController(subscriptionService services.SubscriptionService) http.Handler {
	c := &subscriptionController{
		subscriptionService,
	}

	r := chi.NewRouter()
	r.Post("/", c.createSubscription)
	r.Get("/", c.getAllSubscriptions)
	r.Get("/{id}", c.getSubscriptionByID)
	r.Put("/{id}", c.updateSubscription)
	r.Delete("/{id}", c.deleteSubscription)
	r.Get("/user/{id}", c.getSubscriptionsByUserID)
	r.Put("/{id}/cancel", c.cancelSubscription)
	r.Post("/{id}/renew", c.renewSubscription)
	r.Get("/upcoming-renewals", c.getUpcomingRenewals)

	return r
}

func (c *subscriptionController) createSubscription(w http.ResponseWriter, r *http.Request) {
	subscription := models.SubscriptionRequest{}
	userID, _ := middlewares.GetUserID(r.Context())

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
	userID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.GetSubscriptionByID(r.Context(), subscriptionID, userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) updateSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	subscription := models.SubscriptionUpdateRequest{}
	userID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		ReqBodyObj: &subscription,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.UpdateSubscription(r.Context(), subscriptionID, subscription.ToModel(), userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	userID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		EndpointLogic: func() (any, error) {
			return nil, c.subscriptionService.DeleteSubscription(r.Context(), subscriptionID, userID)
		},
		SuccessCode: http.StatusNoContent,
	})
}

func (c *subscriptionController) getSubscriptionsByUserID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID, _ := middlewares.GetUserID(r.Context())

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
	userID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.CancelSubscription(r.Context(), subscriptionID, userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) renewSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := chi.URLParam(r, "id")
	userID, _ := middlewares.GetUserID(r.Context())

	endpoint.ServeRequest(endpoint.InternalRequest{
		W:          w,
		R:          r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponse(c.subscriptionService.RenewSubscription(r.Context(), subscriptionID, userID))
		},
		SuccessCode: http.StatusOK,
	})
}

func (c *subscriptionController) getUpcomingRenewals(w http.ResponseWriter, r *http.Request) {
	daysParam := r.URL.Query().Get("days")

	endpoint.ServeRequest(endpoint.InternalRequest{
		W: w,
		R: r,
		EndpointLogic: func() (any, error) {
			return endpoint.ToResponseSlice(c.subscriptionService.GetUpcomingRenewals(r.Context(), daysParam))
		},
		SuccessCode: http.StatusOK,
	})
}
