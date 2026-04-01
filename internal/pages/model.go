package pages

type TreeNode struct {
	Name      string     `json:"name"`
	Path      string     `json:"path,omitempty"`
	Title     string     `json:"title,omitempty"`
	IsDir     bool       `json:"is_dir"`
	Children  []TreeNode `json:"children,omitempty"`
	ShowDate  bool       `json:"show_date,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	Published bool       `json:"published"`
}

type PageResponse struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	ViewCount int    `json:"view_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ShowDate  bool   `json:"show_date"`
	Published bool   `json:"published"`
}

type PageRequest struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	ShowDate  *bool  `json:"show_date,omitempty"`
	Published *bool  `json:"published,omitempty"`
}
