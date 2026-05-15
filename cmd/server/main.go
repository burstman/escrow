package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"escrow/internal/config"
	"escrow/internal/db"
	"escrow/internal/fabric"
	"escrow/internal/handler"
	"escrow/internal/middleware"
	"escrow/internal/payment"
	"escrow/internal/storage"
	"escrow/web/templates"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	fc, err := fabric.NewClient(cfg)
	if err != nil {
		log.Fatalf("fabric: %v", err)
	}

	fileStorage, err := storage.NewLocalStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	_ = fileStorage

	var pmtProv payment.Provider
	if cfg.DevMode {
		pmtProv = payment.NewStubProvider()
		log.Println("using stub payment provider (DEV_MODE)")
	} else {
		pmtProv = payment.NewManualProvider(pool, cfg.BusinessBankName, cfg.BusinessRIB, cfg.BusinessAccName)
	}

	ss := handler.NewSessionStore(pool)

	authHandler := handler.NewAuthHandler(pool, ss)
	contractHandler := handler.NewContractHandler(pool, fc, pmtProv, fileStorage)
	fileHandler := handler.NewFileHandler(pool, cfg.UploadDir)
	adminHandler := handler.NewAdminHandler(pool, fc)

	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(userMiddleware(ss))

	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))).ServeHTTP(w, r)
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		user := middleware.GetUser(r)
		if user != nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		templates.Home().Render(r.Context(), w)
	})

	r.Get("/register", authHandler.RegisterForm)
	r.Post("/register", authHandler.Register)
	r.Get("/login", authHandler.LoginForm)
	r.Post("/login", authHandler.Login)
	r.Post("/logout", authHandler.Logout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(ss))
		r.Get("/dashboard", contractHandler.List)
		r.Get("/contracts/new", contractHandler.NewForm)
		r.Post("/contracts", contractHandler.Create)
		r.Get("/contracts/{id}", contractHandler.View)
		r.Post("/contracts/{id}/accept", contractHandler.Accept)
		r.Post("/contracts/{id}/deposit", contractHandler.SubmitDeposit)
		r.Post("/contracts/{id}/deliver", contractHandler.Deliver)
		r.Post("/contracts/{id}/approve", contractHandler.Approve)
		r.Post("/contracts/{id}/revision", contractHandler.Revision)
		r.Post("/contracts/{id}/dispute", contractHandler.Dispute)
		r.Get("/files/{id}/download", fileHandler.Download)

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Get("/", adminHandler.Dashboard)
			r.Get("/payments", adminHandler.Dashboard)
			r.Post("/payments/{id}/confirm", adminHandler.ConfirmPayment)
			r.Post("/payments/{id}/reject", adminHandler.RejectPayment)
			r.Post("/contracts/{id}/resolve", adminHandler.ResolveDispute)
		})
	})

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			rows, err := pool.Query(ctx,
				`SELECT id FROM contracts WHERE state = 'DELIVERED' AND auto_release_at < NOW()`)
			if err != nil {
				log.Printf("auto-release cron query: %v", err)
				continue
			}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					continue
				}
				txID, err := fc.AutoRelease(id)
				if err != nil {
					log.Printf("auto-release contract %s: %v", id, err)
					continue
				}
				_, err = pool.Exec(ctx,
					`UPDATE contracts SET state = 'COMPLETED', completed_at = NOW(), fabric_tx_id = $1 WHERE id = $2`,
					txID, id,
				)
				if err != nil {
					log.Printf("auto-release db update %s: %v", id, err)
				}
				log.Printf("auto-release: contract %s completed", id)
			}
			rows.Close()
			log.Println("auto-release cron: checked")
		}
	}()

	addr := ":" + cfg.Port
	log.Printf("server starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func userMiddleware(ss *handler.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := ss.LoadUser(r)
			if user != nil {
				r = middleware.SetUser(r, user)
			}
			next.ServeHTTP(w, r)
		})
	}
}
