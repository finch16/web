package main

import "database/sql"

type SeriesData struct {
    ID     int
    Name   string
    Values map[int]float64
}

type User struct {
	ID             int
	Username       string
	Email          sql.NullString
	PasswordSHA256 string
}

type Course struct {
	ID          int
	Slug        string
	Title       string
	Language    string
	Description sql.NullString
}

type UserCourse struct {
	UserID   int
	CourseID int
	Role     string
}
