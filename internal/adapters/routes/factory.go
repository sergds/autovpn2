package routes

import "strings"

func NewRouteAdapter(name string) RouteAdapter {
	n := strings.ToLower(name)
	switch n {
	case "keeneticrci":
		return newKeeneticRCI()
	}
	return nil
}
