package observer

import (
"bytes"
_ "embed"
"net/http"
)

//go:embed ui.html
var uiHTMLRaw []byte

//go:embed ui.css
var uiCSSRaw []byte

var uiPage []byte

func init() {
style := append([]byte("<style>"), append(uiCSSRaw, []byte("</style>")...)...)
uiPage = bytes.Replace(uiHTMLRaw, []byte(`<link rel="stylesheet" href="/ui.css">`), style, 1)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Write(uiPage)
}
