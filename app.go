package main

import "net/http"
import "database/sql"
import "github.com/gorilla/mux"
import _ "github.com/mattn/go-sqlite3"

type Application struct {
	jwtTTL int
	jwtSecret string
    DB *sql.DB
}

func start(time int, secret string, storage string, address string) (*Application, error) {
	db, err := sql.Open("sqlite3", storage)
    if err != nil {
		return nil, err
    }

	app := Application{time, secret, db}
	
	r := mux.NewRouter()
	r.HandleFunc("/auth/login", app.loginHandler).Methods("POST")
	r.HandleFunc("/courses", app.coursesHandler).Methods("GET")
	r.HandleFunc("/content", app.contentHandler).Methods("GET")

	if err := http.ListenAndServe(address, r); err != nil {
		return nil, err
	}

	return &app, nil
}

func (app *Application) stop() {
	app.DB.Close()
}
