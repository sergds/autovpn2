package routes

type RouteAdapter interface {
	Authenticate(creds string, endpoint string) error // Creds and endpoint are specific to implementation
	GetRoutes() ([]Route, error)                      // Get all routes from device's routing table.
	AddRoute(route Route, comment string) error       // Add a route. Some fancy routers allow adding text comments to routes for WebUI as well.
	DelRoute(route Route) error                       // Delete a route from the routing table
	SaveConfig() error                                // Some(Probably most) routers don't commit config changes immediately to non-volatile storage. So this should be called before exit.
}
