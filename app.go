package main

import (
	"net/http"
	"github.com/gorilla/mux"
)

type Application struct {
	jwtTTL     int
	jwtSecret  string
	Storage    *Storage
}

func start(time int, secret string, storagePath string, address string) (*Application, error) {
	storage, err := NewStorage(storagePath)
	if err != nil {
		return nil, err
	}

	app := Application{jwtTTL: time, jwtSecret: secret, Storage: storage}
	
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
	app.Storage.Close()
}
