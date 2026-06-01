package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"ledger/internal/config"
)

type Server struct {
	cfg  config.Config
	http *http.Server
}

func NewServer(cfg config.Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)

	return &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:         cfg.Addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", s.cfg.Addr)
		errc <- s.http.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
