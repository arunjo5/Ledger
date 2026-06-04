package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"ledger/internal/config"
	"ledger/internal/ledger"
)

type Server struct {
	cfg  config.Config
	api  http.Handler
	http *http.Server
}

func NewServer(cfg config.Config, store *ledger.Store) *Server {
	h := &handlers{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /health", h.health)

	mux.HandleFunc("POST /accounts", h.createAccount)
	mux.HandleFunc("GET /accounts", h.listAccounts)
	mux.HandleFunc("GET /accounts/{id}", h.getAccount)
	mux.HandleFunc("GET /accounts/{id}/balance", h.accountBalance)
	mux.HandleFunc("GET /accounts/{id}/entries", h.accountEntries)

	mux.HandleFunc("GET /transactions", h.listTransactions)
	mux.HandleFunc("POST /transactions", h.createTransaction)
	mux.HandleFunc("GET /transactions/{id}", h.getTransaction)
	mux.HandleFunc("POST /transactions/{id}/reverse", h.reverseTransaction)

	mux.HandleFunc("GET /overview", h.overview)
	mux.HandleFunc("POST /admin/reconcile", h.reconcile)
	mux.HandleFunc("POST /demo/reset", h.reset)

	// The API serves at the root for tests; in a deployed build it is mounted
	// under /api/v1 and the built frontend is served from StaticDir.
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", handleHealth)
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", mux))
	if cfg.StaticDir != "" {
		root.Handle("/", spaHandler(cfg.StaticDir))
	}

	return &Server{
		cfg: cfg,
		api: mux,
		http: &http.Server{
			Addr:         cfg.Addr,
			Handler:      root,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Handler exposes the API router for tests.
func (s *Server) Handler() http.Handler {
	return s.api
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
