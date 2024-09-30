package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chunghochiu/eth-validator-api/pkg/ethnode"
	"github.com/chunghochiu/eth-validator-api/pkg/relay"
	"github.com/chunghochiu/eth-validator-api/pkg/server"
	"github.com/namsral/flag"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg := bindFlags(fs)

	err := fs.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to parse flagset: %v", err)
	}

	if err := cfg.validate(); err != nil {
		log.Fatalf("failed to validate config: %v", err)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.logLevel)); err != nil {
		log.Fatalf("failed to unmarshal log level: %v", err)
	}

	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: logLevel}))

	exitCode := RunUntilSignal(run, cfg, logger)
	os.Exit(exitCode)
}

func run(ctx context.Context, cfg *config, log *slog.Logger) error {
	log.Info("eth validator api sever is starting.")
	beaconRequester := ethnode.NewBeaconRequester(
		cfg.nodeEndpoint,
		&http.Client{
			Timeout: time.Second * 100,
		},
		log)

	nativeClient, err := ethnode.NewNativeClient(
		ctx,
		cfg.nodeEndpoint,
		log)
	if err != nil {
		log.Error("failed to ethnode.NewNativeClient",
			"error", err)
		return errors.Wrap(err, "failed to ethnode.NewNativeClient")
	}

	relayRequester := relay.NewRequester(
		&http.Client{
			Timeout: time.Second * 30,
		},
		strings.Split(cfg.relayEndpoints, " "),
		log)

	h := server.NewHandler(server.HandlerParam{
		BeaconRequester: beaconRequester,
		NativeClient:    *nativeClient,
		RelayRequester:  relayRequester,
		Logger:          log,
	})

	log.Info("loading validator index map. Please wait around 30 seconds.")
	err = h.UpdateValidatorIndexPubkeyMap(ctx)
	if err != nil {
		log.Error("failed to init UpdateValidatotIndexPubkeyMap",
			"error", err)
		return errors.Wrap(err, "failed to init UpdateValidatotIndexPubkeyMap")
	}
	log.Info("loading is done.")

	r, err := server.MakeRouter(h, &server.RouterConf{
		Logger: log,
	})
	if err != nil {
		log.Error("failed to init router",
			"error", err)
		return errors.Wrap(err, "failed to init router")
	}

	server := http.Server{
		Addr:    cfg.listenAddr,
		Handler: r,
	}
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()

		log.Info("shutting down...")

		timer := time.NewTimer(time.Second * 5)
		go func() {
			timer.Stop()
		}()
		for {
			if timer.Stop() {
				break
			}
		}

		sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = server.Shutdown(sCtx); err != nil {
			return errors.Wrap(err, "failed to Shutdown server")
		}
		return nil
	})

	g.Go(func() error {
		log.Info("Server is listening",
			"listenAddr", cfg.listenAddr)
		if err := server.ListenAndServe(); err != nil {
			return errors.Wrap(err, "server failed to listen and server")
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "error from errgroup")
	}

	return nil
}

// RunUntilSignal detects if any os.Signal to stop the app process
func RunUntilSignal(fn func(context.Context, *config, *slog.Logger) error, cfg *config, log *slog.Logger) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case s := <-sigc:
			log.Info("got os Interrupt",
				"signal: ", s)
			cancel()
		case <-ctx.Done():
		}

		signal.Stop(sigc)
		close(sigc)
	}()

	if err := fn(ctx, cfg, log); err != nil {
		log.Error(errors.Wrap(err, "done: ERROR").Error())
		return 1
	}
	log.Info("done: OK")
	return 0
}
