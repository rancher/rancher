package changes

// ApplyMetrics reports the number of changes during the process.
type ApplyMetrics struct {
	Create int64
	Delete int64
	Patch  int64
	Errors int64
}

// Combine merges two ApplyMetrics together in a new ApplyMetrics value.
func (m1 ApplyMetrics) Combine(m2 *ApplyMetrics) *ApplyMetrics {
	return &ApplyMetrics{
		Create: m1.Create + m2.Create,
		Delete: m1.Delete + m2.Delete,
		Patch:  m1.Patch + m2.Patch,
		Errors: m1.Errors + m2.Errors,
	}
}
