package handlers

type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

type APIResponseAddressBook struct {
	Status        string  `json:"status"`
	ID            string  `json:"id"`
	Balance       float64 `json:"balance"`
	FeePercentage string  `json:"feePercentage"`
	// either BGL or WBGL address to send to
	Address    string `json:"address,omitempty"`
	BGLAddress string `json:"bglAddress,omitempty"`
}

type APIStateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
