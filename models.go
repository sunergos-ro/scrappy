package main

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenshotRequest struct {
	URL               string    `json:"url"`
	Viewport          *Viewport `json:"viewport,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	Format            string    `json:"format,omitempty"`
	Quality           int       `json:"quality,omitempty"`
	DeviceScaleFactor float64   `json:"device_scale_factor,omitempty"`
	WaitMS            int       `json:"wait_ms,omitempty"`
	TimeoutMS         int       `json:"timeout_ms,omitempty"`
}

type ScreenshotResponse struct {
	URL       string `json:"url"`
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	PublicURL string `json:"public_url"`
	Bytes     int    `json:"bytes"`
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	TookMS    int64  `json:"took_ms"`
}

type RenderRequest struct {
	URL       string    `json:"url"`
	Viewport  *Viewport `json:"viewport,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	WaitMS    int       `json:"wait_ms,omitempty"`
	TimeoutMS int       `json:"timeout_ms,omitempty"`
}

type RenderResponse struct {
	URL    string `json:"url"`
	HTML   string `json:"html"`
	TookMS int64  `json:"took_ms"`
}

type MarkdownRequest struct {
	URL              string    `json:"url"`
	Viewport         *Viewport `json:"viewport,omitempty"`
	UserAgent        string    `json:"user_agent,omitempty"`
	WaitMS           int       `json:"wait_ms,omitempty"`
	TimeoutMS        int       `json:"timeout_ms,omitempty"`
	PrimeLazyContent bool      `json:"prime_lazy_content,omitempty"`
}

type MarkdownResponse struct {
	URL      string `json:"url"`
	Markdown string `json:"markdown"`
	TookMS   int64  `json:"took_ms"`
}

type ScaleRequest struct {
	Size int `json:"size"`
}

type ScreenshotOptions struct {
	URL               string
	ViewportWidth     int
	ViewportHeight    int
	UserAgent         string
	WaitMS            int
	TimeoutMS         int
	Format            string
	Quality           int
	DeviceScaleFactor float64
}

type RenderOptions struct {
	URL              string
	ViewportWidth    int
	ViewportHeight   int
	UserAgent        string
	WaitMS           int
	TimeoutMS        int
	PrimeLazyContent bool
}

type ScreenshotResult struct {
	Bytes          []byte
	ContentType    string
	Format         string
	ViewportWidth  int
	ViewportHeight int
}
