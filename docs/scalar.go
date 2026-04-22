package docs

import (
	"bytes"
	"html/template"

	"github.com/gin-gonic/gin"
)

const scalarTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{ .Title }}</title>
</head>
<body>
  <script
    id="api-reference"
    data-url="{{ .SpecURL }}"
    src="https://cdn.jsdelivr.net/npm/@scalar/api-reference">
  </script>
</body>
</html>
`

// ScalarDocsHandler serves the Scalar API reference page.
func ScalarDocsHandler(title, specURL string) gin.HandlerFunc {
	tmpl := template.Must(template.New("scalar").Parse(scalarTemplate))

	return func(c *gin.Context) {
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, map[string]string{
			"Title":   title,
			"SpecURL": specURL,
		})
		c.Data(200, "text/html; charset=utf-8", buf.Bytes())
	}
}
