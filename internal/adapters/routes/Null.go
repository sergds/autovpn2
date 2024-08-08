package routes

// Null Routes Adapter (no-op).
// Usable as a skeleton for new dns adapters for your device/setup.

type NullRoutes struct {
}

func newNullRoutes() *NullRoutes {
	return &NullRoutes{}
}

func (nr *NullRoutes) Authenticate(conf map[string]string) error { return nil }             // Creds and endpoint are specific to implementation
func (nr *NullRoutes) GetRoutes() ([]*Route, error)              { return []*Route{}, nil } // Get all routes from device's routing table.
func (nr *NullRoutes) AddRoute(route Route) error                { return nil }             // Add a route. Some fancy routers allow adding text comments to routes for WebUI as well.
func (nr *NullRoutes) DelRoute(route Route) error                { return nil }             // Delete a route from the routing table
func (nr *NullRoutes) SaveConfig() error                         { return nil }             // Some(Probably most) routers don't commit config changes immediately to non-volatile storage. So this should be called before exit.
