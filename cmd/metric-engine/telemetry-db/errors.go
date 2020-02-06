package telemetry

// some helper types so we can have constant errors. See Dave Cheney's blog on constant errors
type constErr string

func (e constErr) Error() string { return string(e) }
