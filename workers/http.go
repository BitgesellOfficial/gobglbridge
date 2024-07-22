package workers

import (
	"context"
	"crypto/tls"
	"errors"
	"gobglbridge/config"
	"gobglbridge/workers/handlers"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func Worker_HTTP() {
	log.Printf("Starting HTTP service")

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Options("/*", CORSHeaders)

	r.Get("/state", handlers.State)

	r.Get("/balance/bgl", handlers.BalanceBGL)

	r.Get("/balance/eth", handlers.BalanceEth)
	r.Get("/balance/bnb", handlers.BalanceBNB)
	r.Get("/balance/op", handlers.BalanceOP)
	r.Get("/balance/arb", handlers.BalanceArb)

	r.Post("/submit/bgl", handlers.SubmitBGL)
	r.Post("/submit/wbgl", handlers.SubmitWBGL)

	r.Get("/stats/failed", handlers.GetFailedTransactions)
	r.Get("/stats/returnfail", handlers.GetReturnFailTransactions)

	// a bit of logic to prevent directory listing
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		workDir, _ := os.Getwd()
		filesDir := filepath.Join(workDir, "app")
		if _, err := os.Stat(filesDir + r.URL.Path); errors.Is(err, os.ErrNotExist) {
			http.ServeFile(w, r, filepath.Join(filesDir, "index.html"))
		}
		http.ServeFile(w, r, filesDir+r.URL.Path)
	})

	var server *http.Server

	if config.Config.Server.UseSSL {
		cert, _ := tls.LoadX509KeyPair("certchain.pem", "privatekey.pem")
		server = &http.Server{
			Addr:    ":443",
			Handler: r,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		}
	} else {
		server = &http.Server{
			Addr:    ":8080",
			Handler: r,
		}
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if config.Config.Server.UseSSL {
			if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("error listening to: %s", err)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("error listening to: %s", err)
			}
		}
	}()
	log.Print("HTTP service started")

	<-done
	log.Print("HTTP service stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP service shutdown error: %+v", err)
	}
	log.Print("HTTP service shutdown normal")

	// send signal to other threads/workers to exit
	WorkerShutdown = true
}

func CORSHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Origin, X-Requested-With")
}
