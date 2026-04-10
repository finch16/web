package main

type CourseItem struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

type CoursesResponse struct {
	Items []CourseItem `json:"items"`
}
