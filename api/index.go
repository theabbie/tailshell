package handler

import (
	"net/http"

	"tailshell/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	app.Handler(w, r)
}
