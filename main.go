package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"frontend-2-v2/handlers"
	"frontend-2-v2/internal/auth"
	"frontend-2-v2/internal/config"
	"frontend-2-v2/internal/storage"
	"frontend-2-v2/templates"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := buildStore(cfg)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	okta := auth.NewOkta(cfg)
	files := handlers.New(store, templates.Must())
	requireAuth := auth.Require(cfg.JWTSigningKey)

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/auth/login", okta.LoginHandler)
	mux.HandleFunc("/auth/callback", okta.CallbackHandler)
	mux.HandleFunc("/auth/logout", auth.LogoutHandler)

	mux.Handle("/", requireAuth(http.HandlerFunc(files.Home)))
	mux.Handle("/upload", requireAuth(http.HandlerFunc(files.Upload)))
	mux.Handle("/download/", requireAuth(http.HandlerFunc(files.Download)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Server started at http://localhost:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func buildStore(cfg *config.Config) (storage.Store, error) {
	if cfg.UploadsBucket != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return storage.NewS3(ctx, cfg.AWSRegion, cfg.UploadsBucket)
	}
	return storage.NewDisk("uploads")
}
