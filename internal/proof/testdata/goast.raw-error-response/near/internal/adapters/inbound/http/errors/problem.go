package errors

import "net/http"

type problem struct {
	Title string
}

type responder interface {
	JSON(code int, value any)
}

// NotFound is the one door an error leaves through.
func NotFound(c responder) {
	c.JSON(http.StatusNotFound, problem{Title: "not found"})
}
