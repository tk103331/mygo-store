package main

import "github.com/egoist/mygo/ui"

func svg(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	iconSparkle   = svg(`<path d="M8 1.6l1.5 4.1 4.1 1.5-4.1 1.5L8 12.8 6.5 8.7 2.4 7.2l4.1-1.5z"/><path d="M12.6 11.4l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" fill="currentColor" stroke="none"/>`)
	iconHeart     = svg(`<path d="M8 13.6S2.2 10.2 2.2 6.3A3.1 3.1 0 0 1 8 4.5a3.1 3.1 0 0 1 5.8 1.8C13.8 10.2 8 13.6 8 13.6z"/>`)
	iconHeartFill = svg(`<path d="M8 13.6S2.2 10.2 2.2 6.3A3.1 3.1 0 0 1 8 4.5a3.1 3.1 0 0 1 5.8 1.8C13.8 10.2 8 13.6 8 13.6z" fill="currentColor" stroke="none"/>`)
	iconDownload  = svg(`<circle cx="8" cy="8" r="6.2"/><path d="M8 4.8v6M5.4 8.4L8 11l2.6-2.6"/>`)
	iconAgents    = svg(`<rect x="1.6" y="2.4" width="12.8" height="11.2" rx="2.2"/><path d="M4.4 6.2l2.2 1.9-2.2 1.9M8.6 10.6h3"/>`)
	iconDev       = svg(`<path d="M5.6 4.4L2 8l3.6 3.6M10.4 4.4L14 8l-3.6 3.6M9.2 2.8L6.8 13.2"/>`)
	iconAI        = svg(`<path d="M6.2 1.8l.9 2.5 2.5.9-2.5.9-.9 2.5-.9-2.5-2.5-.9 2.5-.9z"/><path d="M11.8 8.2l.7 1.9 1.9.7-1.9.7-.7 1.9-.7-1.9-1.9-.7 1.9-.7z"/>`)
	iconMedia     = svg(`<circle cx="8" cy="8" r="6.2"/><path d="M6.7 5.6l4 2.4-4 2.4z"/>`)
	iconSystem    = svg(`<circle cx="8" cy="8" r="2.2"/><path d="M8 1.6v1.7M8 12.7v1.7M1.6 8h1.7M12.7 8h1.7M3.5 3.5l1.2 1.2M11.3 11.3l1.2 1.2M12.5 3.5l-1.2 1.2M4.7 11.3l-1.2 1.2"/>`)
	iconNotes     = svg(`<path d="M3.4 1.8h6l3.2 3.2v9.2H3.4z"/><path d="M9.2 1.8v3.4h3.4M5.6 8h4.8M5.6 10.4h4.8"/>`)
	iconNetwork   = svg(`<circle cx="8" cy="8" r="6.2"/><path d="M1.8 8h12.4M8 1.8c1.7 1.8 2.5 3.9 2.5 6.2S9.7 12.4 8 14.2C6.3 12.4 5.5 10.3 5.5 8S6.3 3.6 8 1.8z"/>`)
	iconGame      = svg(`<path d="M4.8 4.6h6.4a3.2 3.2 0 0 1 3.1 2.5l.7 3.8a1.6 1.6 0 0 1-2.8 1.3l-1-1.3H5l-1 1.3a1.6 1.6 0 0 1-2.8-1.3l.7-3.8a3.2 3.2 0 0 1 3-2.5z"/><path d="M5.2 7v2.1M4.2 8.1h2.1M10.4 7.7h.1M12 9.1h.1"/>`)
	iconDevices   = svg(`<rect x="4.4" y="4.4" width="7.2" height="7.2" rx="1.4"/><path d="M6.4 1.8v2.6M9.6 1.8v2.6M6.4 11.6v2.6M9.6 11.6v2.6M1.8 6.4h2.6M1.8 9.6h2.6M11.6 6.4h2.6M11.6 9.6h2.6"/>`)
	iconChat      = svg(`<path d="M3.2 2.4h9.6a1.6 1.6 0 0 1 1.6 1.6v5.6a1.6 1.6 0 0 1-1.6 1.6H7.4L4.2 14v-2.8h-1A1.6 1.6 0 0 1 1.6 9.6V4A1.6 1.6 0 0 1 3.2 2.4z"/>`)
	iconBack      = svg(`<path d="M9.8 3.2L5 8l4.8 4.8"/>`)
	iconOpen      = svg(`<path d="M9.2 2.6h4.2v4.2M13.4 2.6L7.2 8.8"/><path d="M12 9.4v3a1.6 1.6 0 0 1-1.6 1.6H3.6A1.6 1.6 0 0 1 2 12.4V5.6A1.6 1.6 0 0 1 3.6 4h3"/>`)
	iconSource    = svg(`<path d="M5.4 4.2L1.9 8l3.5 3.8M10.6 4.2L14.1 8l-3.5 3.8M9.1 2.6L6.9 13.4"/>`)
	iconStar      = svg(`<path d="M8 1.7l1.9 3.9 4.3.6-3.1 3 .7 4.3L8 11.5l-3.8 2 .7-4.3-3.1-3 4.3-.6z" fill="currentColor" stroke="none"/>`)
	iconChevron   = svg(`<path d="M6.2 3.4L10.8 8l-4.6 4.6"/>`)
	iconInfo      = svg(`<circle cx="8" cy="8" r="6.2"/><path d="M8 7.2v4M8 4.8h.01"/>`)
	iconCheck     = svg(`<circle cx="8" cy="8" r="6.2"/><path d="M4.9 8.2l2.1 2.1 4.1-4.4"/>`)
	iconFolder    = svg(`<path d="M1.8 4.3h4.2l1.4 1.6h6.8v7.5H1.8z"/>`)
)
