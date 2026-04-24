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
	Path        string `json:"path"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Description string `json:"description"`
	ViewCount   int    `json:"view_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	ShowDate    bool   `json:"show_date"`
	Published   bool   `json:"published"`
	// RenderedHTML is populated only in the SSR bootstrap payload (main.go),
	// never by the /api/pages handler. Omitempty keeps API responses
	// byte-identical to the pre-SSR shape.
	RenderedHTML string `json:"rendered_html,omitempty"`
}

type PageRequest struct {
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	ShowDate  *bool   `json:"show_date,omitempty"`
	Published *bool   `json:"published,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	Manual    *bool   `json:"manual,omitempty"`
}
