package server

import (
	"net/http"

	"github.com/esc-chula/intania-shop-api/docs"
)

const scalarHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Intania Shop API</title>
</head>
<body>
  <div id="app"></div>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.63.0"></script>
  <script>
    Scalar.createApiReference('#app', {
      url: '/openapi.yaml',
      pageTitle: 'Intania Shop API',
      theme: 'purple',
      hideModels: false,
      hideDownloadButton: false
    })
  </script>
</body>
</html>
`

func openAPIDocument(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(docs.OpenAPI)
}

func scalarReference(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(scalarHTML))
}
