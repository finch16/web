package main

type User struct {
	ID             int
	Username       string
	Email          string
	PasswordSHA256 string
}

type Course struct {
	ID          int
	Slug        string
	Title       string
	Language    string
	Description string
}

type UserCourse struct {
	UserID   int
	CourseID int
	Role     string
}
