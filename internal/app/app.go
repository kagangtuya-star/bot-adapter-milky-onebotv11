package app

import (
	"context"
	"log/slog"

	"milky-onebot11-bridge/internal/bridge"
	"milky-onebot11-bridge/internal/config"
)

type App struct {
	cfg     config.Config
	logger  *slog.Logger
	service *bridge.Service
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	service, err := bridge.NewService(cfg, logger)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:     cfg,
		logger:  logger,
		service: service,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info(
		"starting milky onebot bridge",
		"milky_ws", a.cfg.Milky.WSGateway,
		"milky_rest", a.cfg.Milky.RestGateway,
		"onebot_host", a.cfg.OneBot.Host,
		"onebot_port", a.cfg.OneBot.Port,
		"message_format", a.cfg.Bridge.MessageFormat,
	)
	return a.service.Run(ctx)
}
