package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

type RouterConf struct {
	Logger *slog.Logger
}

// MakeRouter declares the http route for this api server
func MakeRouter(h *Handler, conf *RouterConf) (http.Handler, error) {
	r := chi.NewRouter()

	standardMiddlewares := []func(http.Handler) http.Handler{
		middleware.RequestID,
		middleware.RealIP,
		RequestLogger((conf.Logger)),
		httprate.LimitByIP(100, time.Minute),
	}

	r.Route("/eth/validator", func(r chi.Router) {
		r.Use(standardMiddlewares...)

		r.Get("/blockreward/{slot}", h.GetBlockreward)
		r.Get("/syncduties/{slot}", h.GetSyncduties)
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		render.PlainText(w, r, "OK")
	})

	return r, nil
}
