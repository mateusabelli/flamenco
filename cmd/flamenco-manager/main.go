package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/mattn/go-colorable"
	"github.com/pkg/browser"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/internal/manager/api_impl"
	"projects.blender.org/studio/flamenco/internal/manager/api_impl/dummy"
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/job_deleter"
	"projects.blender.org/studio/flamenco/internal/manager/last_rendered"
	"projects.blender.org/studio/flamenco/internal/manager/local_storage"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/manager/sleep_scheduler"
	"projects.blender.org/studio/flamenco/internal/manager/task_logs"
	"projects.blender.org/studio/flamenco/internal/manager/task_state_machine"
	"projects.blender.org/studio/flamenco/internal/manager/timeout_checker"
	"projects.blender.org/studio/flamenco/internal/own_url"
	"projects.blender.org/studio/flamenco/internal/upnp_ssdp"
	"projects.blender.org/studio/flamenco/pkg/shaman"
	"projects.blender.org/studio/flamenco/pkg/sysinfo"
)

var cliArgs struct {
	version        bool
	writeConfig    bool
	delayResponses bool
	setupAssistant bool
	pprof          bool
}

const (
	developmentWebInterfacePort = 8081

	webappEntryPoint = "index.html"
)

func main() {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	osDetail, err := sysinfo.Description()
	if err != nil {
		osDetail = err.Error()
	}
	log.Info().
		Str("version", appinfo.ApplicationVersion).
		Str("git", appinfo.ApplicationGitHash).
		Str("releaseCycle", appinfo.ReleaseCycle).
		Str("os", runtime.GOOS).
		Str("osDetail", osDetail).
		Str("arch", runtime.GOARCH).
		Msgf("starting %v", appinfo.ApplicationName)

	parseCliArgs()
	if cliArgs.version {
		return
	}

	startFlamenco := true
	for startFlamenco {
		startFlamenco = runFlamencoManager()

		// After the first run, the setup assistant should not be forced any more.
		// If the configuration is still incomplete it can still auto-trigger.
		cliArgs.setupAssistant = false

		if startFlamenco {
			log.Info().
				Str("version", appinfo.ApplicationVersion).
				Str("os", runtime.GOOS).
				Str("arch", runtime.GOARCH).
				Msgf("restarting %v", appinfo.ApplicationName)
		}
	}

	log.Info().Msg("stopping the Flamenco Manager process")
}

// runFlamencoManager starts the entire Flamenco Manager, and only returns after
// it has been completely shut down.
// Returns true if it should be restarted again.
func runFlamencoManager() bool {
	// Load configuration.
	configService := config.NewService()
	err := configService.Load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error().Err(err).Msg("loading configuration")
	}

	if cliArgs.setupAssistant {
		configService.ForceFirstRun()
	}
	isFirstRun, err := configService.IsFirstRun()
	switch {
	case err != nil:
		log.Fatal().Err(err).Msg("unable to determine whether this is the first run of Flamenco or not")
	case isFirstRun:
		log.Info().Msg("This seems to be your first run of Flamenco! A webbrowser will open to help you set things up.")
	}

	if cliArgs.writeConfig {
		err := configService.Save()
		if err != nil {
			log.Error().Err(err).Msg("could not write configuration file")
			os.Exit(1)
		}
		return false
	}

	// TODO: enable TLS via Let's Encrypt.
	listen := configService.Get().Listen
	_, port, _ := net.SplitHostPort(listen)
	log.Info().Str("port", port).Msg("listening")

	// Find our URLs.
	urls, err := own_url.AvailableURLs("http", listen)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to figure out my own URL")
	}

	ssdp := makeAutoDiscoverable(urls)

	// Construct the services.
	persist := openDB(*configService)
	defer persist.Close()

	timeService := clock.New()
	compiler, err := job_compilers.Load(timeService)
	if err != nil {
		log.Fatal().Err(err).Msg("error loading job compilers")
	}

	// Set up the event system.
	eventBroker := eventbus.NewBroker()
	socketio := eventbus.NewSocketIOForwarder()
	eventBroker.AddForwarder(socketio)
	mqttClient := eventbus.NewMQTTForwarder(configService.Get().MQTT.Client)
	if mqttClient != nil {
		eventBroker.AddForwarder(mqttClient)
	}

	localStorage := local_storage.NewNextToExe(configService.Get().LocalManagerStoragePath)
	logStorage := task_logs.NewStorage(localStorage, timeService, eventBroker)

	taskStateMachine := task_state_machine.NewStateMachine(persist, eventBroker, logStorage)
	sleepScheduler := sleep_scheduler.New(timeService, persist, eventBroker)
	lastRender := last_rendered.New(localStorage)

	shamanServer := buildShamanServer(configService, isFirstRun)
	jobDeleter := job_deleter.NewService(persist, localStorage, eventBroker, shamanServer)

	flamenco := api_impl.NewFlamenco(
		compiler, persist, eventBroker, logStorage, configService,
		taskStateMachine, shamanServer, timeService, lastRender,
		localStorage, sleepScheduler, jobDeleter)

	e := buildWebService(flamenco, persist, ssdp, socketio, urls, localStorage)

	timeoutChecker := timeout_checker.New(
		configService.Get().TaskTimeout,
		configService.Get().WorkerTimeout,
		timeService, persist, taskStateMachine, logStorage, eventBroker)

	// The main context determines the lifetime of the application. All
	// long-running goroutines need to keep an eye on this, and stop their work
	// once it closes.
	mainCtx, mainCtxCancel := context.WithCancel(context.Background())

	installSignalHandler(mainCtxCancel)

	if mqttClient != nil {
		mqttClient.Connect(mainCtx)
	}

	// Before doing anything new, clean up in case we made a mess in an earlier run.
	taskStateMachine.CheckStuck(mainCtx)

	// All main goroutines should sync with this waitgroup. Once the waitgroup is
	// done, the main() function will return and the process will stop.
	wg := new(sync.WaitGroup)

	// Run the "last rendered image" processor.
	wg.Add(1)
	go func() {
		defer wg.Done()
		lastRender.Run(mainCtx)
	}()

	// Run a periodic integrity check on the database.
	// When that check fails, the entire application should shut down.
	wg.Add(1)
	go func() {
		defer wg.Done()
		persist.PeriodicIntegrityCheck(mainCtx,
			configService.Get().DBIntegrityCheck,
			mainCtxCancel)
	}()

	// Start the web server.
	wg.Add(1)
	go func() {
		defer wg.Done()

		// No matter how this function ends, if the HTTP server goes down, so does
		// the application.
		defer mainCtxCancel()

		err := runWebService(mainCtx, e, listen)
		if err != nil {
			log.Error().Err(err).Msg("HTTP server error, shutting down the application")
		}
	}()

	// Start the UPnP/SSDP server.
	if ssdp != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ssdp.Run(mainCtx)
		}()
	}

	// Start the timeout checker.
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeoutChecker.Run(mainCtx)
	}()

	// Run the Worker sleep scheduler.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sleepScheduler.Run(mainCtx)
	}()

	// Run the Job Deleter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		jobDeleter.Run(mainCtx)
	}()

	// Log the URLs last, hopefully that makes them more visible / encouraging to go to for users.
	go func() {
		time.Sleep(100 * time.Millisecond)
		logURLs(urls)
	}()

	// Open a webbrowser, but give the web service some time to start first.
	if isFirstRun {
		go openWebbrowser(mainCtx, urls[0])
	}

	// Allow the Flamenco API itself trigger a shutdown as well.
	log.Debug().Msg("waiting for a shutdown request from Flamenco")
	doRestart := flamenco.WaitForShutdown(mainCtx)
	log.Info().Bool("willRestart", doRestart).Msg("going to shut down the service")
	mainCtxCancel()

	wg.Wait()
	log.Info().Bool("willRestart", doRestart).Msg("Flamenco Manager service shut down")

	return doRestart
}

func buildShamanServer(configService *config.Service, isFirstRun bool) api_impl.Shaman {
	if isFirstRun {
		log.Info().Msg("Not starting Shaman storage service, as this is the first run of Flamenco. Configure the shared storage location first.")
		return &dummy.DummyShaman{}
	}
	return shaman.NewServer(configService.Get().Shaman, nil)
}

// openWebbrowser starts a web browser after waiting for 1 second.
// Closing the context aborts the opening of the browser, but doesn't close the
// browser itself if has already started.
func openWebbrowser(ctx context.Context, url url.URL) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(1 * time.Second):
	}

	urlToTry := url.String()
	if err := browser.OpenURL(urlToTry); err != nil {
		log.Error().Err(err).Msgf("unable to open a browser to %s, please open it yourself or try any of the other URLs above", urlToTry)
		return
	}
	log.Info().Str("url", urlToTry).Msgf("opened browser to the Flamenco interface")
}

func parseCliArgs() {
	var quiet, debug, trace bool

	flag.BoolVar(&cliArgs.version, "version", false, "Shows the application version, then exits.")
	flag.BoolVar(&quiet, "quiet", false, "Only log warning-level and worse.")
	flag.BoolVar(&debug, "debug", false, "Enable debug-level logging.")
	flag.BoolVar(&trace, "trace", false, "Enable trace-level logging.")
	flag.BoolVar(&cliArgs.writeConfig, "write-config", false, "Writes configuration to flamenco-manager.yaml, then exits.")
	flag.BoolVar(&cliArgs.delayResponses, "delay", false,
		"Add a random delay to any HTTP responses. This aids in development of Flamenco Manager's web frontend.")
	flag.BoolVar(&cliArgs.setupAssistant, "setup-assistant", false, "Open a webbrowser with the setup assistant.")
	flag.BoolVar(&cliArgs.pprof, "pprof", false, "Expose profiler endpoints on /debug/pprof/.")

	flag.Parse()

	var logLevel zerolog.Level
	switch {
	case trace:
		logLevel = zerolog.TraceLevel
	case debug:
		logLevel = zerolog.DebugLevel
	case quiet:
		logLevel = zerolog.WarnLevel
	default:
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
}

// openDB opens the database or dies.
func openDB(configService config.Service) *persistence.DB {
	dsn := configService.Get().DatabaseDSN
	if dsn == "" {
		log.Fatal().Msg("configure the database in flamenco-manager.yaml")
	}

	dbCtx, dbCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCtxCancel()
	persist, err := persistence.OpenDB(dbCtx, dsn)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("dsn", dsn).
			Msg("error opening database")
	}

	return persist
}

// installSignalHandler spawns a goroutine that handles incoming POSIX signals.
func installSignalHandler(cancelFunc context.CancelFunc) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		for signum := range signals {
			log.Info().Str("signal", signum.String()).Msg("signal received, shutting down")
			cancelFunc()
		}
	}()
}

func makeAutoDiscoverable(urls []url.URL) *upnp_ssdp.Server {
	ssdp, err := upnp_ssdp.NewServer(log.Logger)
	if err != nil {
		strUrls := make([]string, len(urls))
		for idx := range urls {
			strUrls[idx] = urls[idx].String()
		}
		log.Error().Strs("urls", strUrls).Msg("Unable to create UPnP/SSDP server. " +
			"This means that Workers will not be able to automatically find this Manager. " +
			"Run them with the `-manager URL` argument, picking a URL from this list.")
		return nil
	}

	ssdp.AddAdvertisementURLs(urls)
	return ssdp
}

func logURLs(urls []url.URL) {
	log.Info().Int("count", len(urls)).Msg("possible URLs at which to reach Flamenco Manager")
	for _, url := range urls {
		// Don't log this with something like `Str("url", url.String())`, because
		// that puts `url=` in front of the URL. This can interfere with
		// link-detection in the terminal. Having a space in front is much better,
		// as that is guaranteed to count as word-delimiter for double-click
		// word-selection.
		log.Info().Msgf("- %s", url.String())
	}
}
