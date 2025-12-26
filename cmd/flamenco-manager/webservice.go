package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	http_pprof "net/http/pprof"
	"net/url"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ziflex/lecho/v3"

	"projects.blender.org/studio/flamenco/internal/manager/api_impl"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/local_storage"
	"projects.blender.org/studio/flamenco/internal/manager/swagger_ui"
	"projects.blender.org/studio/flamenco/internal/upnp_ssdp"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/web"
)

func buildWebService(
	flamenco api.ServerInterface,
	persist api_impl.PersistenceService,
	ssdp *upnp_ssdp.Server,
	socketio *eventbus.SocketIOForwarder,
	ownURLs []url.URL,
	localStorage local_storage.StorageInfo,
) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// The request should come in fairly quickly, given that Flamenco is intended
	// to run on a local network.
	e.Server.ReadHeaderTimeout = 1 * time.Second
	// e.Server.ReadTimeout is not set, as this is quite specific per request.
	// Shaman file uploads and websocket connections should be allowed to run
	// quite long, whereas other queries should be relatively short.
	//
	// See https://github.com/golang/go/issues/16100 for more info about current
	// limitations in Go that get in our way here.

	// Hook Zerolog onto Echo:
	e.Use(lecho.Middleware(lecho.Config{
		Logger: lecho.From(log.Logger),
	}))

	// Ensure panics when serving a web request won't bring down the server.
	e.Use(middleware.Recover())

	// For development of the web interface, to get a less predictable order of asynchronous requests.
	if cliArgs.delayResponses {
		e.Use(randomDelayMiddleware)
	}

	// Disabled, as it causes issues with "204 No Content" responses.
	// TODO: investigate & file a bug report. Adding the check on an empty slice
	// seems to fix it:
	//
	// func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	// 	if len(b) == 0 {
	// 		return 0, nil
	// 	}
	// 	... original code of the function ...
	// }
	// e.Use(middleware.Gzip())

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: corsOrigins(ownURLs),

		// List taken from https://www.bacancytechnology.com/blog/real-time-chat-application-using-socketio-golang-vuejs/
		AllowHeaders: []string{
			echo.HeaderAccept,
			echo.HeaderAcceptEncoding,
			echo.HeaderAccessControlAllowOrigin,
			echo.HeaderAccessControlRequestHeaders,
			echo.HeaderAccessControlRequestMethod,
			echo.HeaderAuthorization,
			echo.HeaderContentLength,
			echo.HeaderContentType,
			echo.HeaderOrigin,
			echo.HeaderXCSRFToken,
			echo.HeaderXRequestedWith,
			"Cache-Control",
			"Connection",
			"Host",
			"Referer",
			"User-Agent",
			"X-header",
		},
		AllowMethods: []string{"POST", "GET", "OPTIONS", "PUT", "DELETE"},
	}))

	// Load the API definition and enable validation & authentication checks.
	swagger, err := api.GetSwagger()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to get swagger")
	}
	validator := api_impl.SwaggerValidator(*swagger, persist)
	e.Use(validator)
	registerOAPIBodyDecoders()

	// Register routes.
	api.RegisterHandlers(e, flamenco)
	socketio.RegisterHandlers(e)
	swagger_ui.RegisterSwaggerUIStaticFiles(e)
	e.GET("/api/v3/openapi3.json", func(c echo.Context) error {
		return c.JSON(http.StatusOK, swagger)
	})

	// Serve UPnP service descriptions.
	if ssdp != nil {
		e.GET(ssdp.DescriptionPath(), func(c echo.Context) error {
			return c.XMLPretty(http.StatusOK, ssdp.Description(), "  ")
		})
	}

	// Serve static files for the webapp on /app/.
	webAppHandler, err := web.WebAppHandler(webappEntryPoint)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to set up HTTP server for embedded web app")
	}
	e.GET("/app/*", echo.WrapHandler(http.StripPrefix("/app", webAppHandler)))
	e.GET("/app", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})

	// Serve the Blender add-on. It's contained in the static files of the webapp.
	e.GET("/flamenco-addon.zip", echo.WrapHandler(webAppHandler))
	e.GET("/flamenco3-addon.zip", func(c echo.Context) error {
		return c.Redirect(http.StatusPermanentRedirect, "/flamenco-addon.zip")
	})

	// The favicons are also in the static files of the webapp.
	e.GET("/favicon.png", echo.WrapHandler(webAppHandler))
	e.GET("/favicon.ico", echo.WrapHandler(webAppHandler))

	// Serve job-specific files (last-rendered image, task logs) directly from disk.
	log.Info().
		Str("onDisk", localStorage.Root()).
		Str("url", api_impl.JobFilesURLPrefix).
		Msg("serving job-specific files directly from disk")
	e.Static(api_impl.JobFilesURLPrefix, localStorage.Root())

	// Redirect / to the webapp.
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/app/")
	})

	// Register profiler functions.
	if cliArgs.pprof {
		e.GET("/debug/pprof/", echo.WrapHandler(http.HandlerFunc(http_pprof.Index)))
		e.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(http_pprof.Cmdline)))
		e.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(http_pprof.Profile)))
		e.GET("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(http_pprof.Symbol)))
		e.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(http_pprof.Trace)))
		for _, profile := range pprof.Profiles() {
			name := profile.Name()
			e.GET("/debug/pprof/"+name, echo.WrapHandler(http_pprof.Handler(name)))
		}
		log.Info().Msg("profiler debugging info available on /debug/pprof/")
	}

	// Log available routes
	routeLogger := log.Level(zerolog.TraceLevel)
	routeLogger.Trace().Msg("available routes:")
	for _, route := range e.Routes() {
		routeLogger.Trace().Msgf("%7s %s", route.Method, route.Path)
	}

	return e
}

// runWebService runs the Echo server, shutting it down when the context closes.
// If there was any other error, it is returned and the entire server should go down.
func runWebService(ctx context.Context, e *echo.Echo, listen string) error {
	serverStopped := make(chan struct{})
	var httpStartErr error = nil
	var httpShutdownErr error = nil

	go func() {
		defer close(serverStopped)
		err := e.Start(listen)
		if err == http.ErrServerClosed {
			log.Info().Msg("HTTP server shut down")
		} else {
			log.Warn().Err(err).Msg("HTTP server unexpectedly shut down")
			httpStartErr = err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("HTTP server stopping because application is shutting down")

		// Do a clean shutdown of the HTTP server.
		err := e.Shutdown(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("error shutting down HTTP server")
			httpShutdownErr = err
		}

		// Wait until the above goroutine has stopped.
		<-serverStopped

		// Return any error that occurred.
		if httpStartErr != nil {
			return httpStartErr
		}
		return httpShutdownErr

	case <-serverStopped:
		// The HTTP server stopped before the application shutdown was signalled.
		// This is unexpected, so take the entire application down with us.
		if httpStartErr != nil {
			return httpStartErr
		}
		return errors.New("unexpected and unexplained shutdown of HTTP server")
	}
}

// corsOrigins strips everything from the URL that follows the hostname:port, so
// that it's suitable for checking Origin headers of CORS OPTIONS requests.
func corsOrigins(urls []url.URL) []string {
	origins := make([]string, len(urls))

	// TODO: find a way to allow CORS requests during development, but not when
	// running in production.

	for i, url := range urls {
		// Allow the `yarn run dev` webserver do cross-origin requests to this Manager.
		url.Path = ""
		url.Fragment = ""
		url.Host = fmt.Sprintf("%s:%d", url.Hostname(), developmentWebInterfacePort)
		origins[i] = url.String()
	}
	origins = append(origins, fmt.Sprintf("http://localhost:%d", developmentWebInterfacePort))
	log.Debug().Str("origins", strings.Join(origins, " ")).Msg("acceptable CORS origins")
	return origins
}

// randomDelayMiddleware sleeps for a random period of time, as a development tool for frontend work.
func randomDelayMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)

		// Delay the response a bit.
		var duration = int64(rand.NormFloat64()*250 + 125) // in msec
		if duration > 0 {
			if duration > 1000 {
				duration = 1000 // Cap at one second.
			}
			time.Sleep(time.Duration(duration) * time.Millisecond)
		}
		return err
	}
}

func registerOAPIBodyDecoders() {
	// Register "decoders" so that binary data other than
	// "application/octet-stream" can be handled by our OpenAPI library.
	openapi3filter.RegisterBodyDecoder("image/jpeg", openapi3filter.FileBodyDecoder)
	openapi3filter.RegisterBodyDecoder("image/png", openapi3filter.FileBodyDecoder)
}
