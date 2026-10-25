// Package web embeds the HTML templates and static assets served by Spyball.
package web

import "embed"

// Templates holds the html/template files: layout.html plus one file per page.
//
//go:embed templates
var Templates embed.FS

// Static holds the CSS and other assets served under /static/.
//
//go:embed static
var Static embed.FS
