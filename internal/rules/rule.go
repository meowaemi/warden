package rules

type Rule interface {
	Name() string
	Match(input Input) bool
}

type Input struct {
	PodName       string
	Namespace     string
	ContainerName string
	Reason        string
	ExitCode      int32
	RestartCount  int32
}
