package main

import (
	"log"
	"time"
	"net/http"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
)

func (app *Application) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("invalid json: %v", err)
		sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid json"})
		return
	}

	if req.Username == "" || req.Password == "" {
		log.Printf("empty login or password")
		sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid login or password"})
		return
	}

	userID, passwordHash, err := app.Storage.GetUserPasswordHash(req.Username)
	if err != nil {
		log.Printf("error query: %v", err)
		sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid login or password"})
		return
	}

	if userID == 0 || passwordHash != sha256Hex(req.Password) {
		log.Printf("invalid password")
		sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid login or password"})
		return
	}

	token, err := app.createAccessToken(req.Username)
	if err != nil {
		log.Fatal("failed to create token", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"failed to create token"})
		return
	}

	sendJSON(w, http.StatusOK, LoginResponse{token, "Bearer", app.jwtTTL})
}

func (app *Application) createAccessToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Duration(app.jwtTTL) * time.Second).Unix(),
		"iat": time.Now().Unix(),
	})
	return token.SignedString([]byte(app.jwtSecret))
}
