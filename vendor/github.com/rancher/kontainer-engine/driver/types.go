package drivers

const (
	// StringType is the type for string flag
	StringType = "string"
	// BoolType is the type for bool flag
	BoolType = "bool"
	// IntType is the type for int flag
	IntType = "int"
	// StringSliceType is the type for stringSlice flag
	StringSliceType = "stringSlice"
)

// RPCServer defines the interface for a rpc server
type RPCServer interface {
	Serve()
}
