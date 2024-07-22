package handlers

import (
	"net/http"
)

// prev. bridge implementation compatibility
func State(w http.ResponseWriter, r *http.Request) {
	//ctx := context.Background()
	responseJSON(w, &APIStateResponse{
		Status: "ok",
	}, http.StatusOK)
}
