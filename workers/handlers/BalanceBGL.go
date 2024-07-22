package handlers

import (
	"fmt"
	"gobglbridge/BGLRPC"
	"log"
	"net/http"
)

func BalanceBGL(w http.ResponseWriter, r *http.Request) {

	balance, err := BGLRPC.GetClient().GetBalance()
	if err != nil {
		log.Printf("Error getting BGL balance: %s", err.Error())
		responsePlain(w, []byte("error"), 500)
	} else {
		responsePlain(w, []byte(fmt.Sprintf("%d", int64(balance))), 200)
	}
}
