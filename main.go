package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

func main() {
	serveMux := http.NewServeMux()

	cfg := &apiConfig{}
	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(appHandler))
	serveMux.HandleFunc("GET /api/healthz", healthzHandlerFunc)
	serveMux.HandleFunc("GET /admin/metrics", cfg.getMetricsHandler)
	serveMux.HandleFunc("POST /admin/reset", cfg.resetMetricsHandler)
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
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
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w,
		`<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`,
		cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hits reset to 0")
	cfg.fileserverHits.Store(0)
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
	return nil
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type requestParams struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(r.Body)
	params := &requestParams{}
	err := decoder.Decode(params)
	if err != nil {
		fmt.Println("Error decoding request json body")
		return
	}
	type responseParams struct {
		CleanedBody string `json:"cleaned_body"`
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	response := responseParams{CleanedBody: cleanedInput(params.Body)}
	respondWithJSON(w, 200, response)

}

func cleanedInput(input string) string {
	banned := map[string]bool{"kerfuffle": true, "sharbert": true, "fornax": true}
	words := strings.Split(input, " ")
	for i, word := range words {
		if banned[strings.ToLower(word)] {
			words[i] = "****"
		}
	}
	output := strings.Join(words, " ")
	return output
}
