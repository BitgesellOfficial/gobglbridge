package handlers

import (
	"net/http"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	//ctx := context.Background()
	responseJSON(w, &APIResponse{
		Status: "ok",
	}, http.StatusOK)
}
