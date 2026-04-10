package main

import (
	"fmt"
	"log"
	"sort"
	"net/http"
)

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

	user, err := app.Storage.GetUserByUsername(username)
	if err != nil {
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid token"})
		return
	}

	course, err := app.Storage.GetCourseBySlug(courseSlug)
	if err != nil {
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid course slug"})
		return
	}

	_, err = app.Storage.GetUserCourseRole(user.ID, course.ID)
	if err != nil {
		sendJSON(w, http.StatusUnauthorized, ErrorResponse{"Invalid course slug"})
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
	menuNodes, pagesMap, pagesSlugMap, err := app.Storage.GetCourseMenuAndPages(courseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get menu and pages: %w", err)
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

	blocksMap, err := app.Storage.GetPageBlocks(pageIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get page blocks: %w", err)
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

			if block.Text != "" {
				pageBlock.Text = &block.Text
			}
			if block.URL != "" {
				pageBlock.URL = &block.URL
			}
			if block.Caption != "" {
				pageBlock.Caption = &block.Caption
			}

			if block.Type == "video" {
				pageBlock.Autoplay = &block.Autoplay
				pageBlock.Loop = &block.Loop
				pageBlock.ShowControls = &block.ShowControls
			}

			if block.Type == "chart" && bd.Chart != nil {
				pageBlock.ChartType = &bd.Chart.ChartType
				if bd.Chart.Title != "" {
					pageBlock.Title = &bd.Chart.Title
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
		childrenMap[node.ParentID] = append(childrenMap[node.ParentID], node)
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
			} else if child.PageID > 0 {
				for slug, page := range pagesMap {
					if page.ID == child.PageID {
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
