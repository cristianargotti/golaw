package http

import "net/http"

type body struct {
	Message string
}

type responder interface {
	JSON(code int, value any)
}

// NotFound answers the error outside the errors package.
func NotFound(c responder) {
	c.JSON(http.StatusNotFound, body{Message: "not found"})
}
