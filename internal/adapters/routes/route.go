package routes

// Represents an IP route for our needs. Based on keenetic representation, which is probably bad for compatability.
type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Comment     string `json:"comment"`
}
