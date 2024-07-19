package rpc

const (
	STATUS_OTHER   int32 = iota // Unused for now
	STATUS_FETCHIP              // Use when resolving ips
	STATUS_DNS                  // Use when using dns adapter
	STATUS_ROUTES               // Use when using routes adapter
	STATUS_NOTIFY               // Status text gets put into current step name on client
	STATUS_ERROR                // For errors, status text is essentially the error message
)

const (
	UNDO_STATUS_OTHER  int32 = iota // Unused for now
	UNDO_STATUS_DNS                 // When deleting DNS entries for playbook
	UNDO_STATUS_ROUTES              // When removing routes
	UNDO_STATUS_NOTIFY              // Status text gets put into current step name on client
	UNDO_STATUS_ERROR               // For errors, status text is essentially the error message
)
