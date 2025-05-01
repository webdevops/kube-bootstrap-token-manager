package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/azuresdk/prometheus/tracing"

	"github.com/webdevops/kube-bootstrap-token-manager/config"
	"github.com/webdevops/kube-bootstrap-token-manager/manager"
)

const (
	Author    = "webdevops.io"
	UserAgent = "k8s-boottkn-mgmt/"
)

var (
	argparser *flags.Parser
	Opts      config.Opts

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()
	initLogger()

	logger.Infof("starting kube-bootstrap-token-manager v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	logger.Info(string(Opts.GetJson()))
	initSystem()

	manager := manager.KubeBootstrapTokenManager{
		Opts:      Opts,
		Version:   gitTag,
		Logger:    logger,
		UserAgent: fmt.Sprintf(`%s/%s`, UserAgent, gitTag),
	}

	manager.Init()
	manager.Start()

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func startHttpServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	mux.Handle("/metrics", tracing.RegisterAzureMetricAutoClean(promhttp.Handler()))

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	logger.Fatal(srv.ListenAndServe())
}
