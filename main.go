package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/GeorgeHat/chirpy/internal/auth"
	"github.com/GeorgeHat/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error Connecting to the database")
		os.Exit(1)
	}
	dbQueries := database.New(db)
	cfg := &apiConfig{db: dbQueries}

	serveMux := http.NewServeMux()

	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(appHandler))
	serveMux.HandleFunc("GET /api/healthz", healthzHandlerFunc)
	serveMux.HandleFunc("GET /admin/metrics", cfg.getMetricsHandler)
	serveMux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	serveMux.HandleFunc("GET /api/chirps", cfg.getChirpsHandler)
	serveMux.HandleFunc("GET /api/chirps/{id}", cfg.getChirpHandler)
	serveMux.HandleFunc("POST /api/chirps", cfg.createChirpHandler)
	serveMux.HandleFunc("POST /api/users", cfg.createUserHandler)
	serveMux.HandleFunc("POST /api/login", cfg.loginHandler)
	server := new(http.Server)
	server.Addr = ":8080"
	server.Handler = serveMux
	log.Fatal(server.ListenAndServe())

}

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Post struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func healthzHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK\n"))
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("PLATFORM") == "dev" {
		cfg.fileserverHits.Store(0)
		err := cfg.db.DeleteAllUsers(r.Context())
		if err != nil {
			respondWithError(w, 500, "Error deleting users")
			return
		}
		w.WriteHeader(200)

	} else {
		w.WriteHeader(403)
	}
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

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	type requestParams struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	params := &requestParams{}
	err := decoder.Decode(params)
	if err != nil {
		fmt.Println("Error decoding request json body")
		return
	}

	if !validateChirp(params.Body) {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	post, err := cfg.db.CreatePost(r.Context(), database.CreatePostParams{Body: params.Body, UserID: params.UserID})
	if err != nil {
		respondWithError(w, 500, "Error creating post")
		return
	}
	response := Post{
		ID:        post.ID,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
		Body:      censorInput(post.Body),
		UserID:    post.UserID,
	}
	respondWithJSON(w, 201, response)

}

func censorInput(input string) string {
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

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type requestParams struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := &requestParams{}
	err := decoder.Decode(params)
	if err != nil {
		respondWithError(w, 500, "Error decoding json")
		return
	}

	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}

	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{Email: params.Email, HashedPassword: hash})
	if err != nil {
		respondWithError(w, 500, "Error creating user")
		return
	}
	reponse := User{ID: user.ID, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Email: user.Email}
	respondWithJSON(w, 201, reponse)

}

func validateChirp(chirp string) bool {
	if len(chirp) > 140 {
		return false
	}
	return true
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	posts, err := cfg.db.GetAllPosts(r.Context())
	if err != nil {
		respondWithError(w, 500, "Error getting chirps from database")
		return
	}
	response := make([]Post, len(posts))
	for i, post := range posts {
		response[i] = Post{ID: post.ID, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt, Body: post.Body, UserID: post.UserID}
	}
	respondWithJSON(w, 200, response)

}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	id_string := r.PathValue("id")
	id, err := uuid.Parse(id_string)
	if err != nil {
		respondWithError(w, 500, "Error parsing id")
		return
	}
	post, err := cfg.db.GetPostById(r.Context(), id)
	if err != nil {
		respondWithError(w, 404, "Error getting chirp")
		return
	}

	response := Post{ID: post.ID, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt, Body: post.Body, UserID: post.UserID}
	respondWithJSON(w, 200, response)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	type requestParams struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := &requestParams{}
	err := decoder.Decode(params)
	if err != nil {
		respondWithError(w, 500, "Error decoding json")
		return
	}
	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	match, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	if !match {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	response := User{ID: user.ID, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Email: user.Email}
	respondWithJSON(w, 200, response)
}
