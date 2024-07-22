package handlers

import (
	"gobglbridge/redis"
	"net/http"
)

func GetFailedTransactions(w http.ResponseWriter, r *http.Request) {

	failedTxs, err := redis.FindAllBridgeOperationsByStatus("failed")

	if err != nil {
		responseJSON(w, nil, 500)
		return
	}

	responseJSON(w, failedTxs, 200)
}

func GetReturnFailTransactions(w http.ResponseWriter, r *http.Request) {

	failedTxs, err := redis.FindAllBridgeOperationsByStatus("returnfail")

	if err != nil {
		responseJSON(w, nil, 500)
		return
	}

	responseJSON(w, failedTxs, 200)
}
