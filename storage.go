package main

import "database/sql"

type storage struct {
    DB *sql.DB
}

func open(path string) (*storage) {
	db, err := sql.Open("sqlite3", path)
    if err != nil {
		return nil, err
	}
	return &storage{DB: db}, nil
}
