package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nestify/backend/internal/auth"
	"nestify/backend/internal/config"
	"nestify/backend/internal/executor"
	"nestify/backend/internal/httpapi"
	"nestify/backend/internal/pathbrowse"
	"nestify/backend/internal/store/sqlite"
)

func Run() error {
	env := config.LoadEnv()

	store, err := sqlite.Open(env)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			log.Printf("close store error: %v", closeErr)
		}
	}()

	srv := &http.Server{
		Addr: env.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			Env:        env,
			Store:      store,
			Sessions:   auth.NewSessionManager(24 * time.Hour),
			PathBrowse: pathbrowse.New(env.BrowseRoots),
			Executor:   executor.NewService(),
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Printf("nestify backend listening on %s, sqlite=%s", env.HTTPAddr, env.DBPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
