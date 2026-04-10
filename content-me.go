package main

import "database/sql"

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
    ParentID  sql.NullInt64
    Title     string
    SortOrder int
    PageID    sql.NullInt64
}

type PageBlockDB struct {
    ID           int
    PageID       int
    Position     int
    Type         string
    Text         sql.NullString
    URL          sql.NullString
    Caption      sql.NullString
    Autoplay     sql.NullInt64
    Loop         sql.NullInt64
    ShowControls sql.NullInt64
}

type ChartBlockDB struct {
    BlockID   int
    ChartType string
    Title     sql.NullString
}

type ChartXLabelDB struct {
    BlockID int
    Idx     int
    Label   string
}

type ChartSeriesDB struct {
    ID        int
    BlockID   int
    Name      string
    SortOrder int
}

type ChartSeriesValueDB struct {
    SeriesID int
    XIdx     int
    YValue   float64
}
