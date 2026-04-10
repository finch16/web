package main

import "log"
import "fmt"
import "strings"
import "net/http"
import "github.com/golang-jwt/jwt/v5"

func (app *Application) coursesHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	username, err := getUsername(authHeader, app.jwtSecret);
	if err != nil {
		log.Printf("Invalid token");
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
		return
	}

	user, err := app.getUser(username); 
	if err != nil {
		log.Printf("Database error: %v", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Database error"})
		return
	}

	rows, err := app.DB.Query(
		"SELECT c.slug, c.title, c.language FROM courses c " +
		"INNER JOIN user_courses uc ON uc.course_id = c.id " +
		"WHERE uc.user_id = ? ORDER BY c.id",
		user.ID,
	)
	
	if err != nil {
		log.Printf("Failed to query courses: %v", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Failed to fetch courses"})
		return
	}
	defer rows.Close()

	var courses []CourseItem
	for rows.Next() {
		var course CourseItem
		if err := rows.Scan(&course.Slug, &course.Title, &course.Language); err != nil {
			log.Printf("Failed to scan course: %v", err)
			continue
		}
		courses = append(courses, course)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Rows iteration error: %v", err)
		sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Failed to fetch courses"})
		return
	}

	sendJSON(w, http.StatusOK, CoursesResponse{courses})
}

func getUsername(authHeader string, secret string) (string, error){
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

func (app *Application) getUser(username string) (*User, error) {
    var user User
    err := app.DB.QueryRow(
        "SELECT id, username, email, password_sha256 FROM users WHERE username = ?",
        username,
    ).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordSHA256)
    if err != nil {
        return nil, err
    }
    return &user, nil
}
