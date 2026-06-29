package app

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/containeroo/httpgrace/server"
	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/cli"
	"github.com/containeroo/overdue/internal/handler"
	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notify"
	"github.com/containeroo/overdue/internal/routes"
	"github.com/containeroo/overdue/internal/scheduler"
	"github.com/containeroo/overdue/internal/service"
	"github.com/containeroo/tinyflags"
)

// Run parses configuration, wires dependencies, and runs the HTTP server.
func Run(
	ctx context.Context,
	version, commit string,
	args []string,
	stdOut, stdErr io.Writer,
	templateFS fs.FS,
) error {
	cfg, err := cli.ParseArgs(args, version)
	if err != nil {
		if tinyflags.IsHelpRequested(err) || tinyflags.IsVersionRequested(err) {
			_, _ = fmt.Fprint(stdOut, err)
			return nil
		}
		_, _ = fmt.Fprintln(stdErr, err)
		return err
	}

	logger := logging.SetupLogger(cfg.Debug, cfg.LogFormat, stdOut)
	setupLog := logger.With("component", "setup")
	setupLog.Info("starting overdue", "version", version, "commit", commit)

	setupLog.Info(
		"check-in receiver configured",
		"listenAddr", cfg.ListenAddr.String(),
		"routePrefix", cfg.RoutePrefix,
		"siteRoot", cfg.SiteRoot,
		"name", cfg.CheckIn.Name,
		"path", cfg.CheckIn.Path,
		"expectedEvery", cfg.CheckIn.ExpectedEvery.String(),
		"alertingDelay", cfg.CheckIn.AlertingDelay.String(),
		"startActive", cfg.CheckIn.StartActive,
		"allowGETCheckIn", cfg.CheckIn.AllowGET,
		"responseDetails", cfg.ResponseDetails,
		"initialPhase", monitor.PhaseScheduled,
		"notifications", len(cfg.Notifications.Webhooks)+len(cfg.Notifications.Emails),
	)

	if len(cfg.OverriddenValues) > 0 {
		logger.Info(
			"cli overrides",
			"overrides", cfg.OverriddenValues,
		)
	}

	receivers, notificationRouter, err := notify.ReceiversFromConfig(
		templateFS,
		cfg.Notifications,
		logger.With("component", "notify"),
	)
	if err != nil {
		setupLog.Error("notification setup error", "error", err)
		return err
	}

	notifyManager, err := kit.NewManager(receivers, logger.With("component", "notify"))
	if err != nil {
		setupLog.Error("notification setup error", "error", err)
		return err
	}
	setupLog.Info("configured notifiers", "receivers", len(notifyManager.Receivers()))

	ctx, stop := server.SignalContext(ctx)
	defer stop()
	if err := notifyManager.Start(ctx); err != nil {
		setupLog.Error("notification start error", "error", err)
		return err
	}

	mon := monitor.New(
		cfg.CheckIn.Name,
		cfg.CheckIn.ExpectedEvery,
		cfg.CheckIn.AlertingDelay,
		logger.With("component", "monitor"),
	)

	reg := metrics.NewRegistry()
	sched := scheduler.New(mon, notifyManager, notificationRouter, reg, logger.With("component", "scheduler"))
	if cfg.CheckIn.StartActive {
		activatedAt := time.Now()
		sched.RecordCheckInContext(ctx, activatedAt)
		setupLog.Info(
			"check-in monitor activated at startup",
			"activatedAt", activatedAt,
			"expectedBy", activatedAt.Add(cfg.CheckIn.ExpectedEvery),
			"alertingAt", activatedAt.Add(cfg.CheckIn.ExpectedEvery+cfg.CheckIn.AlertingDelay),
		)
	}
	sched.Run(ctx)

	svc := service.NewCheckIn(sched, reg)
	api := handler.NewAPI(
		cfg.AuthToken,
		svc,
		reg,
		cfg.ResponseDetails,
		version,
		commit,
		logger.With("component", "api"),
	)

	router := routes.NewRouter(cfg.CheckIn.Path, cfg.RoutePrefix, cfg.CheckIn.AllowGET, api)
	if err := server.Run(
		ctx,
		cfg.ListenAddr.String(),
		router,
		logger.With("component", "server"),
	); err != nil {
		setupLog.Error("server run", "listenAddr", cfg.ListenAddr.String(), "error", err)
		return err
	}

	return nil
}
