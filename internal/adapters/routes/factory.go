package routes

import "strings"

// Creates adapter from config name. Basically a simple Factory method.
func NewRouteAdapter(name string) RouteAdapter {
	n := strings.ToLower(name)
	switch n {
	case "keeneticrci":
		return newKeeneticRCI()
	case "null":
		return newNullRoutes()
	default:
		return newNullRoutes()
	}
	return nil
}
