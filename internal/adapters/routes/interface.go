package routes

type RouteAdapter interface {
	Authenticate(creds string, endpoint string) bool // Creds and endpoint are specific to implementation
	GetRoutes() []Route                              // Get all routes from device's routing table.
	AddRoute(route Route, comment string) bool       // Add a route. Some fancy routers allow adding text comments to routes for WebUI as well.
	DelRoute(route Route) bool                       // Delete a route from the routing table
	SaveConfig() bool                                // Some(Probably most) routers don't commit config changes immediately to non-volatile storage. So this should be called before exit.
}
