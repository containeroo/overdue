package app

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/containeroo/httpgrace/server"
	kit "github.com/containeroo/notifykit/notify"
	"github.com/containeroo/overdue/internal/flag"
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
	flags, err := flag.ParseArgs(args, version)
	if err != nil {
		if tinyflags.IsHelpRequested(err) || tinyflags.IsVersionRequested(err) {
			_, _ = fmt.Fprint(stdOut, err)
			return nil
		}
		_, _ = fmt.Fprintln(stdErr, err)
		return err
	}

	logger := logging.SetupLogger(flags.Debug, flags.LogFormat, stdOut)
	setupLog := logger.With("component", "setup")
	setupLog.Info("starting overdue", "version", version, "commit", commit)

	setupLog.Info(
		"check-in receiver configured",
		"listenAddr", flags.ListenAddr.String(),
		"routePrefix", flags.RoutePrefix,
		"siteRoot", flags.SiteRoot,
		"name", flags.CheckIn.Name,
		"path", flags.CheckIn.Path,
		"expectedEvery", flags.CheckIn.ExpectedEvery.String(),
		"alertingDelay", flags.CheckIn.AlertingDelay.String(),
		"startActive", flags.CheckIn.StartActive,
		"allowGETCheckIn", flags.CheckIn.AllowGET,
		"responseDetails", flags.ResponseDetails,
		"initialPhase", monitor.PhaseScheduled,
		"notifications", len(flags.Notifications.Webhooks)+len(flags.Notifications.Emails),
	)
	setupLog.Debug("notifications", "webhooks", flags.Notifications.Webhooks, "emails", flags.Notifications.Emails)

	receivers, notificationRouter, err := notify.ReceiversFromConfig(
		templateFS,
		flags.Notifications,
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
		flags.CheckIn.Name,
		flags.CheckIn.ExpectedEvery,
		flags.CheckIn.AlertingDelay,
		logger.With("component", "monitor"),
	)

	reg := metrics.NewRegistry()
	sched := scheduler.New(mon, notifyManager, notificationRouter, reg, logger.With("component", "scheduler"))
	if flags.CheckIn.StartActive {
		activatedAt := time.Now()
		sched.RecordCheckInContext(ctx, activatedAt)
		setupLog.Info(
			"check-in monitor activated at startup",
			"activatedAt", activatedAt,
			"expectedBy", activatedAt.Add(flags.CheckIn.ExpectedEvery),
			"alertingAt", activatedAt.Add(flags.CheckIn.ExpectedEvery+flags.CheckIn.AlertingDelay),
		)
	}
	sched.Run(ctx)

	svc := service.NewCheckIn(sched, reg)
	api := handler.NewAPI(
		flags.AuthToken,
		svc,
		reg,
		flags.ResponseDetails,
		version,
		commit,
		logger.With("component", "api"),
	)

	router := routes.NewRouter(flags.CheckIn.Path, flags.RoutePrefix, flags.CheckIn.AllowGET, api)
	if err := server.Run(
		ctx,
		flags.ListenAddr.String(),
		router,
		logger.With("component", "server"),
	); err != nil {
		setupLog.Error("server run", "listenAddr", flags.ListenAddr.String(), "error", err)
		return err
	}

	return nil
}
