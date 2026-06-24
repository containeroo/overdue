package app

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/containeroo/httpgrace/server"
	"github.com/containeroo/overdue/internal/flag"
	"github.com/containeroo/overdue/internal/handler"
	"github.com/containeroo/overdue/internal/logging"
	"github.com/containeroo/overdue/internal/metrics"
	"github.com/containeroo/overdue/internal/monitor"
	"github.com/containeroo/overdue/internal/notification/notifier"
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
		"listenAddr", flags.ListenAddr,
		"routePrefix", flags.RoutePrefix,
		"publicURL", flags.PublicURL,
		"name", flags.CheckIn.Name,
		"path", flags.CheckIn.Path,
		"expectedEvery", flags.CheckIn.ExpectedEvery.String(),
		"alertingDelay", flags.CheckIn.AlertingDelay.String(),
		"startActive", flags.CheckIn.StartActive,
		"responseDetails", flags.ResponseDetails,
		"initialPhase", monitor.PhaseScheduled,
	)

	if err := notifier.ValidateRuntimeTemplates(
		templateFS,
		flags.Notify,
		flags.CheckIn.Name,
		flags.CheckIn.ExpectedEvery,
		flags.CheckIn.AlertingDelay,
	); err != nil {
		setupLog.Error("notification template validation error", "error", err)
		return err
	}

	reg := metrics.NewRegistry()

	notify, err := notifier.New(
		templateFS,
		flags.Notify,
		logger.With("component", "notify"),
	)
	if err != nil {
		setupLog.Error("notifier setup error", "error", err)
		return err
	}

	ctx, stop := server.SignalContext(ctx)
	defer stop()

	mon := monitor.New(
		flags.CheckIn.Name,
		flags.CheckIn.ExpectedEvery,
		flags.CheckIn.AlertingDelay,
		logger.With("component", "monitor"),
	)

	sched := scheduler.New(mon, notify, reg, logger.With("component", "scheduler"))
	if flags.CheckIn.StartActive {
		activatedAt := time.Now()
		sched.RecordCheckIn(activatedAt)
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

	router := routes.NewRouter(flags.CheckIn.Path, flags.RoutePrefix, api)
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
