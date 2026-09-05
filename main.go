package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	serveMux := http.NewServeMux()

	cfg := &apiConfig{}
	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(appHandler))
	serveMux.HandleFunc("GET /api/healthz", healthzHandlerFunc)
	serveMux.HandleFunc("GET /api/metrics", cfg.getMetricsHandler)
	serveMux.HandleFunc("POST /api/reset", cfg.resetMetricsHandler)
	server := new(http.Server)
	server.Addr = ":8080"
	server.Handler = serveMux
	log.Fatal(server.ListenAndServe())

}

func healthzHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hits: %d", cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hits reset to 0")
	cfg.fileserverHits.Store(0)
}
