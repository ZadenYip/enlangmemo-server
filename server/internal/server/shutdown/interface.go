package shutdown

type GracefulShutdowner interface {
	GracefulShutdown() error
}
