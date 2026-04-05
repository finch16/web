package main

import (
	"flag"
	"fmt"
	"log"
    "net/http"
    "os"
	"strings"
	"crypto/sha256"
    "encoding/hex"
	"database/sql"
	"encoding/json"
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    ExpiresIn   int    `json:"expires_in"`
}

type ErrorResponse struct {
    Error string `json:"error"`
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

type CourseItem struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

type CoursesResponse struct {
	Items []CourseItem `json:"items"`
}

func sha256Hex(password string) string {
    hash := sha256.Sum256([]byte(password))
    return hex.EncodeToString(hash[:])
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

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

type Application struct {
	jwtTTL int
	jwtSecret string
    DB *sql.DB
}

func (app *Application) createAccessToken(username string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": username,
        "exp": time.Now().Add(time.Duration(app.jwtTTL) * time.Second).Unix(),
        "iat": time.Now().Unix(),
    })
    return token.SignedString([]byte(app.jwtSecret))
}

func (app *Application) loginHandler(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid json"})
        return
    }

    if req.Username == "" || req.Password == "" {
        sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid login or password"})
        return
    }

    var userID int
    var passwordHash string
    
	err := app.DB.QueryRow(
        "SELECT id, password_sha256 FROM users WHERE username = ?",
        req.Username,
    ).Scan(&userID, &passwordHash)
    
    if err != nil {
        sendJSON(w, http.StatusBadRequest, ErrorResponse{"invalid login or password"})
        return
    }

    if passwordHash != sha256Hex(req.Password) {
        sendJSON(w, http.StatusUnauthorized, ErrorResponse{"invalid login or password"})
        return
    }

    token, err := app.createAccessToken(req.Username)
    if err != nil {
		log.Fatal(err)
        sendJSON(w, http.StatusInternalServerError, ErrorResponse{"failed to create token"})
        return
    }

    sendJSON(w, http.StatusOK, LoginResponse{token, "Bearer", app.jwtTTL})
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

func (app *Application) coursesHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	username, err := getUsername(authHeader, app.jwtSecret);

	if err != nil {
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
		return
	}

	var user User
	err = app.DB.QueryRow(
		"SELECT id, username, email, password_sha256 FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordSHA256)
	
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSON(w, http.StatusUnauthorized, ErrorResponse{"User not found"})
		} else {
			log.Printf("Database error: %v", err)
			sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Database error"})
		}
		return
	}

	rows, err := app.DB.Query(
		"SELECT c.slug, c.title, c.language FROM courses c ",
		"INNER JOIN user_courses uc ON uc.course_id = c.id ",
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

func (app *Application) contentHandler(w http.ResponseWriter, r *http.Request) {
    authHeader := r.Header.Get("Authorization")
    username, err := getUsername(authHeader, app.jwtSecret)
    if err != nil {
        sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
        return
    }

    courseSlug := r.URL.Query().Get("course")
    if courseSlug == "" {
        sendJSON(w, http.StatusBadRequest, ErrorResponse{"Missing query param: course"})
        return
    }

    var user User
    err = app.DB.QueryRow(
        "SELECT id, username, email, password_sha256 FROM users WHERE username = ?",
        username,
    ).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordSHA256)
    
    if err != nil {
        sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
        return
    }

    var course Course
    err = app.DB.QueryRow(
        "SELECT id, slug, title, language, COALESCE(description, '') FROM courses WHERE slug = ?",
        courseSlug,
    ).Scan(&course.ID, &course.Slug, &course.Title, &course.Language, &course.Description)
    
    if err != nil {
        sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid course slog"})
        return
    }

    var userCourse UserCourse
    err = app.DB.QueryRow(
        "SELECT user_id, course_id, role FROM user_courses WHERE user_id = ? AND course_id = ?",
        user.ID, course.ID,
    ).Scan(&userCourse.UserID, &userCourse.CourseID, &userCourse.Role)
    
    if err != nil {
        sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid course slog"})
        return
    }

    content, err := app.buildCourseContentJSON(course.ID)
    if err != nil {
        log.Printf("Failed to build course content: %v", err)
        sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Failed to load course content"})
        return
    }

    content.AppTitle = course.Title
    sendJSON(w, http.StatusOK, content)
}

func (app *Application) buildCourseContentJSON(courseID int) (*ContentResponse, error) {
    menuRows, err := app.DB.Query(
        "SELECT id, course_id, COALESCE(parent_id, 0), title, sort_order, COALESCE(page_id, 0) "+
        "FROM course_menu_nodes WHERE course_id = ? ORDER BY sort_order, id",
        courseID,
    )
    if err != nil {
        return nil, err
    }
    defer menuRows.Close()

    var menuNodes []CourseMenuNode
    for menuRows.Next() {
        var node CourseMenuNode
        var parentID int
        var pageID int
        if err := menuRows.Scan(&node.ID, &node.CourseID, &parentID, &node.Title, &node.SortOrder, &pageID); err != nil {
            return nil, err
        }
        if parentID > 0 {
            node.ParentID = sql.NullInt64{Int64: int64(parentID), Valid: true}
        }
        if pageID > 0 {
            node.PageID = sql.NullInt64{Int64: int64(pageID), Valid: true}
        }
        menuNodes = append(menuNodes, node)
    }

    pagesRows, err := app.DB.Query(
        "SELECT id, course_id, slug, title, sort_order FROM course_pages WHERE course_id = ?",
        courseID,
    )
    if err != nil {
        return nil, err
    }
    defer pagesRows.Close()

    pagesMap := make(map[int]CoursePage)
    pagesSlugMap := make(map[string]CoursePage)
    for pagesRows.Next() {
        var page CoursePage
        if err := pagesRows.Scan(&page.ID, &page.CourseID, &page.Slug, &page.Title, &page.SortOrder); err != nil {
            return nil, err
        }
        pagesMap[page.ID] = page
        pagesSlugMap[page.Slug] = page
    }

    pageIDs := make([]int, 0, len(pagesMap))
    for id := range pagesMap {
        pageIDs = append(pageIDs, id)
    }
    
    blocksMap := make(map[int][]PageBlockDB)
    for _, pageID := range pageIDs {
        blocksRows, err := app.DB.Query(
            "SELECT id, page_id, position, type, text, url, caption, autoplay, loop, show_controls ",
            "FROM page_blocks WHERE page_id = ? ORDER BY position, id",
            pageID,
        )
        if err != nil {
            return nil, err
        }
        
        var blocks []PageBlockDB
        for blocksRows.Next() {
            var block PageBlockDB
            if err := blocksRows.Scan(
                &block.ID, &block.PageID, &block.Position, &block.Type,
                &block.Text, &block.URL, &block.Caption,
                &block.Autoplay, &block.Loop, &block.ShowControls,
            ); err != nil {
                blocksRows.Close()
                return nil, err
            }
            blocks = append(blocks, block)
        }
        blocksRows.Close()
        blocksMap[pageID] = blocks
    }

    menu := app.buildMenuTree(menuNodes, pagesSlugMap)
    
    pages := make(map[string]PageContent)
    for slug, page := range pagesSlugMap {
        blocks := blocksMap[page.ID]
        pageContent := PageContent{
            Title:  page.Title,
            Blocks: make([]PageBlock, 0, len(blocks)),
        }
        
        for _, block := range blocks {
            pageBlock := PageBlock{
                ID:   block.Position,
                Type: block.Type,
            }
            
            if block.Text.Valid {
                pageBlock.Text = &block.Text.String
            }
            if block.URL.Valid {
                pageBlock.URL = &block.URL.String
            }
            if block.Caption.Valid {
                pageBlock.Caption = &block.Caption.String
            }
            
            if block.Type == "video" {
                if block.Autoplay.Valid {
                    autoplay := block.Autoplay.Int64 == 1
                    pageBlock.Autoplay = &autoplay
                }
                if block.Loop.Valid {
                    loop := block.Loop.Int64 == 1
                    pageBlock.Loop = &loop
                }
                if block.ShowControls.Valid {
                    showControls := block.ShowControls.Int64 == 1
                    pageBlock.ShowControls = &showControls
                }
            }
            
            if block.Type == "chart" {
                var chart ChartBlockDB
                err := app.DB.QueryRow(
                    "SELECT block_id, chart_type, title FROM chart_blocks WHERE block_id = ?",
                    block.ID,
                ).Scan(&chart.BlockID, &chart.ChartType, &chart.Title)
                
                if err == nil {
                    pageBlock.ChartType = &chart.ChartType
                    if chart.Title.Valid {
                        pageBlock.Title = &chart.Title.String
                    }
                    
                    labelsRows, err := app.DB.Query(
                        "SELECT idx, label FROM chart_x_labels WHERE block_id = ? ORDER BY idx",
                        block.ID,
                    )
                    if err == nil {
                        var xLabels []string
                        for labelsRows.Next() {
                            var idx int
                            var label string
                            if err := labelsRows.Scan(&idx, &label); err == nil {
                                xLabels = append(xLabels, label)
                            }
                        }
                        labelsRows.Close()
                        if len(xLabels) > 0 {
                            pageBlock.XLabels = xLabels
                        }
                    }
                    
                    seriesRows, err := app.DB.Query(
                        "SELECT id, name, sort_order FROM chart_series WHERE block_id = ? ORDER BY sort_order, id",
                        block.ID,
                    )
                    if err == nil {
                        var seriesList []ChartSeries
                        for seriesRows.Next() {
                            var series ChartSeriesDB
                            if err := seriesRows.Scan(&series.ID, &series.Name, &series.SortOrder); err == nil {
                                valuesRows, err := app.DB.Query(
                                    "SELECT x_idx, y_value FROM chart_series_values WHERE series_id = ? ORDER BY x_idx",
                                    series.ID,
                                )
                                if err == nil {
                                    var values []float64
                                    for valuesRows.Next() {
                                        var xIdx int
                                        var yValue float64
                                        if err := valuesRows.Scan(&xIdx, &yValue); err == nil {
                                            values = append(values, yValue)
                                        }
                                    }
                                    valuesRows.Close()
                                    seriesList = append(seriesList, ChartSeries{
                                        Name:   series.Name,
                                        Values: values,
                                    })
                                }
                            }
                        }
                        seriesRows.Close()
                        if len(seriesList) > 0 {
                            pageBlock.Series = seriesList
                        }
                    }
                }
            }
            
            pageContent.Blocks = append(pageContent.Blocks, pageBlock)
        }
        pages[slug] = pageContent
    }
    
    return &ContentResponse{
        Menu:  menu,
        Pages: pages,
    }, nil
}

func (app *Application) buildMenuTree(nodes []CourseMenuNode, pagesMap map[string]CoursePage) []MenuNode {
    childrenMap := make(map[int][]CourseMenuNode)
    for _, node := range nodes {
        parentID := 0
        if node.ParentID.Valid {
            parentID = int(node.ParentID.Int64)
        }
        childrenMap[parentID] = append(childrenMap[parentID], node)
    }
    
    for _, children := range childrenMap {
        for i := 0; i < len(children)-1; i++ {
            for j := i + 1; j < len(children); j++ {
                if children[i].SortOrder > children[j].SortOrder {
                    children[i], children[j] = children[j], children[i]
                }
            }
        }
    }
    
    var buildTree func(parentID int) []MenuNode
    buildTree = func(parentID int) []MenuNode {
        var result []MenuNode
        children := childrenMap[parentID]
        for _, child := range children {
            node := MenuNode{
                Title: child.Title,
            }
            
            grandchildren := buildTree(child.ID)
            if len(grandchildren) > 0 {
                node.Children = grandchildren
            } else if child.PageID.Valid {
                for slug, page := range pagesMap {
                    if page.ID == int(child.PageID.Int64) {
                        node.PageId = slug
                        break
                    }
                }
            }
            
            result = append(result, node)
        }
        return result
    }
    
    return buildTree(0)
}

func main() {
	time := flag.Int("t", 3600, "time")
	address := flag.String("b", "localhost:5000", "address")
	secret := flag.String("s", "", "secret")
	store := flag.String("d", "./store.db", "path to database file")
	flag.Parse()

	db, err := sql.Open("sqlite3", *store)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
		os.Exit(1)
    }
    defer db.Close()

	app := &Application{*time, *secret, db}

	r := mux.NewRouter()
	r.HandleFunc("/auth/login", app.loginHandler).Methods("POST")
	r.HandleFunc("/courses", app.coursesHandler).Methods("GET")
	r.HandleFunc("/content", app.contentHandler).Methods("GET")

	http.ListenAndServe(*address, r);
}
