package v1

// NetworkingStackPreference is the desired interface for networking calls.
// This is used when rendering probes, allowing the user to decide whether "127.0.0.1", "::1", or "localhost" is appropriate for determining probe status.
type NetworkingStackPreference = string

const (
	// DualStackPreference signifies a dual stack networking strategy, defaulting "localhost" for communication on the
	// loopback interface
	DualStackPreference = NetworkingStackPreference("dual")

	// SingleStackIPv4Preference signifies a single stack IPv4 networking strategy, defaulting "127.0.0.1" for
	// communication on the loopback interface
	SingleStackIPv4Preference = NetworkingStackPreference("ipv4")

	// SingleStackIPv6Preference signifies a single stack IPv6 networking strategy, defaulting "::1" for
	// communication on the loopback interface
	SingleStackIPv6Preference = NetworkingStackPreference("ipv6")

	// DefaultStackPreference is the stack preference used when no preference is defined, or is invalid. Defaults to
	// "127.0.0.1" to support existing behavior.
	DefaultStackPreference = SingleStackIPv4Preference
)
