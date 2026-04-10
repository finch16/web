package main

import (
	"log"
	"fmt"
	"strings"
	"net/http"
	"github.com/golang-jwt/jwt/v5"
)

func (app *Application) coursesHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	username, err := getUsername(authHeader, app.jwtSecret)
	if err != nil {
		log.Printf("Invalid token")
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
		return
	}

	user, err := app.Storage.GetUserByUsername(username)
	if err != nil {
		log.Printf("Database error: %v", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Database error"})
		return
	}

	courses, err := app.Storage.GetCoursesByUserID(user.ID)
	if err != nil {
		log.Printf("Failed to query courses: %v", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Failed to fetch courses"})
		return
	}

	sendJSON(w, http.StatusOK, CoursesResponse{courses})
}

func getUsername(authHeader string, secret string) (string, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return "", fmt.Errorf("Invalid authorization header")
	}

	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})

	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("Invalid token")
	}

	username, ok := claims["sub"].(string)
	if !ok || username == "" {
		return "", fmt.Errorf("Invalid token")
	}

	return username, nil
}
