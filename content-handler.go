package main

import "fmt"
import "log"
import "sort"
import "strings"
import "net/http"
import "database/sql"

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

	user, err := app.getUser(username); 
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
        log.Printf("Failed to build course content: %v", err, course.ID)
        sendJSON(w, http.StatusInternalServerError, ErrorResponse{"Failed to load course content"})
        return
    }

    content.AppTitle = course.Title
    sendJSON(w, http.StatusOK, content)
}

func (app *Application) buildCourseContentJSON(courseID int) (*ContentResponse, error) {
    rows, err := app.DB.Query(`
        SELECT 
            'menu' as type,
            mn.id, mn.course_id, COALESCE(mn.parent_id, 0), mn.title, mn.sort_order, COALESCE(mn.page_id, 0),
            NULL as slug, NULL as page_title, NULL as page_sort_order
        FROM course_menu_nodes mn
        WHERE mn.course_id = ?
        
        UNION ALL
        
        SELECT 
            'page' as type,
            cp.id, cp.course_id, 0 as parent_id, cp.title, cp.sort_order, 0 as page_id,
            cp.slug, cp.title as page_title, cp.sort_order as page_sort_order
        FROM course_pages cp
        WHERE cp.course_id = ?
        
        ORDER BY type, sort_order, id
    `, courseID, courseID)
    
    if err != nil {
        return nil, fmt.Errorf("failed to query menu and pages: %w", err)
    }
    defer rows.Close()

    var menuNodes []CourseMenuNode
    pagesMap := make(map[int]CoursePage)
    pagesSlugMap := make(map[string]CoursePage)

    for rows.Next() {
        var nodeType string
        var id, courseID, parentID, sortOrder, pageID int
        var title string
        var slug, pageTitle sql.NullString
        var pageSortOrder sql.NullInt64

        if err := rows.Scan(
            &nodeType,
            &id, &courseID, &parentID, &title, &sortOrder, &pageID,
            &slug, &pageTitle, &pageSortOrder,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan row: %w", err)
        }

        if nodeType == "menu" {
            node := CourseMenuNode{
                ID:        id,
                CourseID:  courseID,
                Title:     title,
                SortOrder: sortOrder,
            }
            if parentID > 0 {
                node.ParentID = sql.NullInt64{Int64: int64(parentID), Valid: true}
            }
            if pageID > 0 {
                node.PageID = sql.NullInt64{Int64: int64(pageID), Valid: true}
            }
            menuNodes = append(menuNodes, node)
        } else {
            page := CoursePage{
                ID:        id,
                CourseID:  courseID,
                Slug:      slug.String,
                Title:     pageTitle.String,
                SortOrder: int(pageSortOrder.Int64),
            }
            pagesMap[page.ID] = page
            pagesSlugMap[page.Slug] = page
        }
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %w", err)
    }

    if len(pagesMap) == 0 {
        return &ContentResponse{
            Menu:  app.buildMenuTree(menuNodes, pagesSlugMap),
            Pages: make(map[string]PageContent),
        }, nil
    }

    pageIDs := make([]int, 0, len(pagesMap))
    for id := range pagesMap {
        pageIDs = append(pageIDs, id)
    }

    placeholders := strings.Repeat("?,", len(pageIDs))
    placeholders = placeholders[:len(placeholders)-1]
    
    query := fmt.Sprintf(`
        SELECT 
            pb.page_id,
            pb.id as block_id,
            pb.position,
            pb.type,
            pb.text,
            pb.url,
            pb.caption,
            pb.autoplay,
            pb.loop,
            pb.show_controls,
            COALESCE(cb.chart_type, '') as chart_type,
            COALESCE(cb.title, '') as chart_title,
            COALESCE(cxl.idx, -1) as label_idx,
            COALESCE(cxl.label, '') as label,
            COALESCE(cs.id, -1) as series_id,
            COALESCE(cs.name, '') as series_name,
            COALESCE(cs.sort_order, 0) as series_order,
            COALESCE(csv.x_idx, -1) as x_idx,
            COALESCE(csv.y_value, 0) as y_value
        FROM page_blocks pb
        LEFT JOIN chart_blocks cb ON cb.block_id = pb.id
        LEFT JOIN chart_x_labels cxl ON cxl.block_id = pb.id
        LEFT JOIN chart_series cs ON cs.block_id = pb.id
        LEFT JOIN chart_series_values csv ON csv.series_id = cs.id
        WHERE pb.page_id IN (%s)
        ORDER BY pb.page_id, pb.position, pb.id, cs.sort_order, cs.id, cxl.idx, csv.x_idx
    `, placeholders)

    args := make([]interface{}, len(pageIDs))
    for i, id := range pageIDs {
        args[i] = id
    }

    blockRows, err := app.DB.Query(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to query blocks: %w", err)
    }
    defer blockRows.Close()

    type BlockData struct {
        Block   PageBlockDB
        Chart   *ChartBlockDB
        XLabels map[int]string
        Series  map[int]*SeriesData
    }
    
    blocksMap := make(map[int][]*BlockData)
    currentBlockID := -1
    var currentBlockData *BlockData

    for blockRows.Next() {
        var pageID, blockID, position int
        var blockType string
        var text, url, caption sql.NullString
        var autoplay, loop, showControls sql.NullInt64
        var chartType, chartTitle string
        var labelIdx int
        var label string
        var seriesID int
        var seriesName string
        var seriesOrder int
        var xIdx int
        var yValue float64

        if err := blockRows.Scan(
            &pageID, &blockID, &position, &blockType,
            &text, &url, &caption, &autoplay, &loop, &showControls,
            &chartType, &chartTitle,
            &labelIdx, &label,
            &seriesID, &seriesName, &seriesOrder,
            &xIdx, &yValue,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan block row: %w", err)
        }

        if blockID != currentBlockID {
            if currentBlockData != nil {
                blocksMap[currentBlockData.Block.PageID] = append(blocksMap[currentBlockData.Block.PageID], currentBlockData)
            }
            
            currentBlockID = blockID
            currentBlockData = &BlockData{
                Block: PageBlockDB{
                    ID:           blockID,
                    PageID:       pageID,
                    Position:     position,
                    Type:         blockType,
                    Text:         text,
                    URL:          url,
                    Caption:      caption,
                    Autoplay:     autoplay,
                    Loop:         loop,
                    ShowControls: showControls,
                },
                XLabels: make(map[int]string),
                Series:  make(map[int]*SeriesData),
            }
            
            if chartType != "" {
                currentBlockData.Chart = &ChartBlockDB{
                    BlockID:   blockID,
                    ChartType: chartType,
                    Title:     sql.NullString{String: chartTitle, Valid: chartTitle != ""},
                }
            }
        }

        if labelIdx >= 0 && label != "" {
            currentBlockData.XLabels[labelIdx] = label
        }
        
        if seriesID >= 0 && seriesName != "" {
            if _, exists := currentBlockData.Series[seriesID]; !exists {
                currentBlockData.Series[seriesID] = &SeriesData{
                    ID:     seriesID,
                    Name:   seriesName,
                    Values: make(map[int]float64),
                }
            }
            
            if xIdx >= 0 {
                currentBlockData.Series[seriesID].Values[xIdx] = yValue
            }
        }
    }
    
    if currentBlockData != nil {
        blocksMap[currentBlockData.Block.PageID] = append(blocksMap[currentBlockData.Block.PageID], currentBlockData)
    }

    pages := make(map[string]PageContent)
    for slug, page := range pagesSlugMap {
        pageContent := PageContent{
            Title:  page.Title,
            Blocks: make([]PageBlock, 0),
        }
        
        blockDatas := blocksMap[page.ID]
        for _, bd := range blockDatas {
            block := bd.Block
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
            
            if block.Type == "chart" && bd.Chart != nil {
                pageBlock.ChartType = &bd.Chart.ChartType
                if bd.Chart.Title.Valid {
                    pageBlock.Title = &bd.Chart.Title.String
                }
                
                if len(bd.XLabels) > 0 {
                    maxIdx := 0
                    for idx := range bd.XLabels {
                        if idx > maxIdx {
                            maxIdx = idx
                        }
                    }
                    xLabels := make([]string, maxIdx+1)
                    for idx, label := range bd.XLabels {
                        xLabels[idx] = label
                    }
                    pageBlock.XLabels = xLabels
                }
                
                if len(bd.Series) > 0 {
                    seriesList := make([]ChartSeries, 0, len(bd.Series))
                    
                    seriesIDs := make([]int, 0, len(bd.Series))
                    for id := range bd.Series {
                        seriesIDs = append(seriesIDs, id)
                    }
                    sort.Ints(seriesIDs)
                    
                    for _, id := range seriesIDs {
                        series := bd.Series[id]
                        if len(series.Values) > 0 {
                            maxXIdx := 0
                            for idx := range series.Values {
                                if idx > maxXIdx {
                                    maxXIdx = idx
                                }
                            }
                            values := make([]float64, maxXIdx+1)
                            for idx, val := range series.Values {
                                values[idx] = val
                            }
                            seriesList = append(seriesList, ChartSeries{
                                Name:   series.Name,
                                Values: values,
                            })
                        }
                    }
                    if len(seriesList) > 0 {
                        pageBlock.Series = seriesList
                    }
                }
            }
            
            pageContent.Blocks = append(pageContent.Blocks, pageBlock)
        }
        
        pages[slug] = pageContent
    }

    return &ContentResponse{
        Menu:  app.buildMenuTree(menuNodes, pagesSlugMap),
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
		sort.Slice(children, func(i, j int) bool {
			return children[i].SortOrder < children[j].SortOrder
		})
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
