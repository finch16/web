package main

import (
	"database/sql"
	"fmt"
	"strings"
	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(path string) (*Storage, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	return &Storage{DB: db}, nil
}

func (s *Storage) Close() error {
	return s.DB.Close()
}

// User methods
func (s *Storage) GetUserByUsername(username string) (*User, error) {
	var user User
	var email sql.NullString
	err := s.DB.QueryRow(
		"SELECT id, username, email, password_sha256 FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &email, &user.PasswordSHA256)
	if err != nil {
		return nil, err
	}
	if email.Valid {
		user.Email = email.String
	}
	return &user, nil
}

func (s *Storage) GetUserPasswordHash(username string) (int, string, error) {
	var userID int
	var passwordHash string
	err := s.DB.QueryRow(
		"SELECT id, password_sha256 FROM users WHERE username = ?",
		username,
	).Scan(&userID, &passwordHash)
	return userID, passwordHash, err
}

// Course methods
func (s *Storage) GetCoursesByUserID(userID int) ([]CourseItem, error) {
	rows, err := s.DB.Query(
		"SELECT c.slug, c.title, c.language FROM courses c "+
			"INNER JOIN user_courses uc ON uc.course_id = c.id "+
			"WHERE uc.user_id = ? ORDER BY c.id",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var courses []CourseItem
	for rows.Next() {
		var course CourseItem
		if err := rows.Scan(&course.Slug, &course.Title, &course.Language); err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (s *Storage) GetCourseBySlug(slug string) (*Course, error) {
	var course Course
	var description sql.NullString
	err := s.DB.QueryRow(
		"SELECT id, slug, title, language, COALESCE(description, '') FROM courses WHERE slug = ?",
		slug,
	).Scan(&course.ID, &course.Slug, &course.Title, &course.Language, &description)
	if err != nil {
		return nil, err
	}
	if description.Valid {
		course.Description = description.String
	}
	return &course, nil
}

func (s *Storage) GetUserCourseRole(userID, courseID int) (*UserCourse, error) {
	var userCourse UserCourse
	err := s.DB.QueryRow(
		"SELECT user_id, course_id, role FROM user_courses WHERE user_id = ? AND course_id = ?",
		userID, courseID,
	).Scan(&userCourse.UserID, &userCourse.CourseID, &userCourse.Role)
	if err != nil {
		return nil, err
	}
	return &userCourse, nil
}

// Content methods
// Content methods
func (s *Storage) GetCourseMenuAndPages(courseID int) ([]CourseMenuNode, map[int]CoursePage, map[string]CoursePage, error) {
	rows, err := s.DB.Query(`
		SELECT 
			'menu' as type,
			mn.id, mn.course_id, COALESCE(mn.parent_id, 0), mn.title, mn.sort_order, COALESCE(mn.page_id, 0),
			COALESCE(NULL, '') as slug, 
			COALESCE(NULL, '') as page_title, 
			COALESCE(NULL, 0) as page_sort_order
		FROM course_menu_nodes mn
		WHERE mn.course_id = ?
		
		UNION ALL
		
		SELECT 
			'page' as type,
			cp.id, cp.course_id, 0 as parent_id, cp.title, cp.sort_order, 0 as page_id,
			COALESCE(cp.slug, '') as slug, 
			COALESCE(cp.title, '') as page_title, 
			COALESCE(cp.sort_order, 0) as page_sort_order
		FROM course_pages cp
		WHERE cp.course_id = ?
		
		ORDER BY type, sort_order, id
	`, courseID, courseID)

	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query menu and pages: %w", err)
	}
	defer rows.Close()

	var menuNodes []CourseMenuNode
	pagesMap := make(map[int]CoursePage)
	pagesSlugMap := make(map[string]CoursePage)

	for rows.Next() {
		var nodeType string
		var id, courseID, parentID, sortOrder, pageID int
		var title string
		var slug, pageTitle string
		var pageSortOrder int

		if err := rows.Scan(
			&nodeType,
			&id, &courseID, &parentID, &title, &sortOrder, &pageID,
			&slug, &pageTitle, &pageSortOrder,
		); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if nodeType == "menu" {
			node := CourseMenuNode{
				ID:        id,
				CourseID:  courseID,
				Title:     title,
				SortOrder: sortOrder,
				ParentID:  0,
				PageID:    0,
			}
			if parentID > 0 {
				node.ParentID = parentID
			}
			if pageID > 0 {
				node.PageID = pageID
			}
			menuNodes = append(menuNodes, node)
		} else {
			page := CoursePage{
				ID:        id,
				CourseID:  courseID,
				Slug:      slug,
				Title:     pageTitle,
				SortOrder: pageSortOrder,
			}
			pagesMap[page.ID] = page
			if slug != "" {
				pagesSlugMap[slug] = page
			}
		}
	}

	return menuNodes, pagesMap, pagesSlugMap, rows.Err()
}

func (s *Storage) GetPageBlocks(pageIDs []int) (map[int][]*BlockData, error) {
	if len(pageIDs) == 0 {
		return make(map[int][]*BlockData), nil
	}

	placeholders := strings.Repeat("?,", len(pageIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
		SELECT 
			pb.page_id,
			pb.id as block_id,
			pb.position,
			pb.type,
			COALESCE(pb.text, '') as text,
			COALESCE(pb.url, '') as url,
			COALESCE(pb.caption, '') as caption,
			COALESCE(pb.autoplay, 0) as autoplay,
			COALESCE(pb.loop, 0) as loop,
			COALESCE(pb.show_controls, 0) as show_controls,
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

	blockRows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}
	defer blockRows.Close()

	blocksMap := make(map[int][]*BlockData)
	currentBlockID := -1
	var currentBlockData *BlockData

	for blockRows.Next() {
		var pageID, blockID, position int
		var blockType string
		var text, url, caption string
		var autoplay, loop, showControls int
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
					Autoplay:     autoplay == 1,
					Loop:         loop == 1,
					ShowControls: showControls == 1,
				},
				XLabels: make(map[int]string),
				Series:  make(map[int]*SeriesData),
			}

			if chartType != "" {
				currentBlockData.Chart = &ChartBlockDB{
					BlockID:   blockID,
					ChartType: chartType,
					Title:     chartTitle,
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

	return blocksMap, blockRows.Err()
}
