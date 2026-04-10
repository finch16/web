package main

type CoursePage struct {
	ID        int
	CourseID  int
	Slug      string
	Title     string
	SortOrder int
}

type CourseMenuNode struct {
	ID        int
	CourseID  int
	ParentID  int
	Title     string
	SortOrder int
	PageID    int
}

type PageBlockDB struct {
	ID           int
	PageID       int
	Position     int
	Type         string
	Text         string
	URL          string
	Caption      string
	Autoplay     bool
	Loop         bool
	ShowControls bool
}

type ChartBlockDB struct {
	BlockID   int
	ChartType string
	Title     string
}

type SeriesData struct {
	ID     int
	Name   string
	Values map[int]float64
}

type BlockData struct {
	Block   PageBlockDB
	Chart   *ChartBlockDB
	XLabels map[int]string
	Series  map[int]*SeriesData
}
