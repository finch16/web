package main

type ContentRequest struct {
    Course string `json:"course"`
}

type MenuNode struct {
    Title    string     `json:"title,omitempty"`
    PageId   string     `json:"pageId,omitempty"`
    Children []MenuNode `json:"children,omitempty"`
}

type PageBlock struct {
    ID           int            `json:"id"`
    Type         string         `json:"type"`
    Text         *string        `json:"text,omitempty"`
    URL          *string        `json:"url,omitempty"`
    Caption      *string        `json:"caption,omitempty"`
    Autoplay     *bool          `json:"autoplay,omitempty"`
    Loop         *bool          `json:"loop,omitempty"`
    ShowControls *bool          `json:"showControls,omitempty"`
    ChartType    *string        `json:"chartType,omitempty"`
    Title        *string        `json:"title,omitempty"`
    XLabels      []string       `json:"xLabels,omitempty"`
    Series       []ChartSeries  `json:"series,omitempty"`
}

type ChartSeries struct {
    Name   string    `json:"name"`
    Values []float64 `json:"values"`
}

type PageContent struct {
    Title  string      `json:"title"`
    Blocks []PageBlock `json:"blocks"`
}

type ContentResponse struct {
    AppTitle string                 `json:"appTitle"`
    Menu     []MenuNode             `json:"menu"`
    Pages    map[string]PageContent `json:"pages"`
}
