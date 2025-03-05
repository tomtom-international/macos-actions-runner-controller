package probe

type Result string

const (
	Success Result = "success"

	Failure Result = "failure"

	Unknown Result = "unknown"
)
