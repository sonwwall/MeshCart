package component

import (
	logx "meshcart/app/log"
	"meshcart/gateway/config"

	otelprovider "github.com/hertz-contrib/obs-opentelemetry/provider"
)

func InitLogger(cfg config.Config) {
	if err := logx.Init(logx.Config{
		Service: cfg.App.Name,
		Env:     cfg.App.Env,
		Level:   cfg.Log.Level,
		LogDir:  cfg.Log.LogDir,
	}); err != nil {
		panic(err)
	}
}

func InitOpenTelemetry(cfg config.Config) otelprovider.OtelProvider {
	options := []otelprovider.Option{
		otelprovider.WithServiceName(cfg.App.Name),
		otelprovider.WithDeploymentEnvironment(cfg.App.Env),
		otelprovider.WithExportEndpoint(cfg.Telemetry.Endpoint),
	}
	if cfg.Telemetry.Insecure {
		options = append(options, otelprovider.WithInsecure())
	}
	return otelprovider.NewOpenTelemetryProvider(options...)
}
