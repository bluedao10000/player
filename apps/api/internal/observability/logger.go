package observability

import "go.uber.org/zap"

// NewLogger also captures existing standard-library log calls from workers.
func NewLogger(service string) (*zap.Logger, func(), error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, nil, err
	}
	logger = logger.With(zap.String("service", service))
	restore := zap.RedirectStdLog(logger)
	return logger, func() { restore(); _ = logger.Sync() }, nil
}
