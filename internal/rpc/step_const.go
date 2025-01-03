package rpc

// contains enums for steps that we can build tasks of.
const (
	STEP_LIST         = "list"         // List playbooks
	STEP_FETCHIP      = "fetchip"      // Use when resolving ips
	STEP_DNS          = "dns"          // Use when using dns adapter
	STEP_ROUTES       = "routes"       // Use when using routes adapter
	STEP_NOTIFY       = "notify"       // STATE text gets put into current step name on client
	STEP_ERROR        = "error"        // Terminates executors! For errors, STATE text is essentially the error message.
	STEP_PUSH_SUMMARY = "push_summary" // Push this string into client's summary. Summary is shown at the end of operation.
	STEP_LOCK_ADD     = "lock_add"
	STEP_PREP_CTX     = "prep_ctx"
)

const (
	UNDO_STEP_DNS    = "undo_dns"    // When deleting DNS entries for playbook
	UNDO_STEP_ROUTES = "undo_routes" // When removing routes
)

// Describe to the poor user tf we are doing right now.
func DescribeState(state string) string {
	switch state {
	case STEP_LIST:
		return "List of playbooks"
	case STEP_FETCHIP:
		return "Fetching IP Addresses of hosts"
	case STEP_DNS:
		return "Applying DNS records"
	case STEP_ROUTES:
		return "Adding static routes"
	case STEP_NOTIFY:
		return ""
	case STEP_ERROR:
		return "During execution of the task following failed:"
	case STEP_PUSH_SUMMARY:
		return ""
	case UNDO_STEP_DNS:
		return "Undoing DNS records"
	case UNDO_STEP_ROUTES:
		return "Undoing static routes"
	case STEP_LOCK_ADD:
		return "Locking playbook and adding to DB"
	case STEP_PREP_CTX:
		return "Preparing for operation"
	default:
		return "" // no idea, something custom. Maybe we don't need these enums, if 40% of tasks will hit default case here.
	}
}
