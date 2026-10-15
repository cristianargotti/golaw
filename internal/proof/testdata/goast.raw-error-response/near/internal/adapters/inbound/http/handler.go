package http

import "net/http"

type body struct {
	Message string
}

type responder interface {
	JSON(code int, value any)
}

// Found answers a success and a legacy envelope the contract names.
func Found(c responder, legacy bool) {
	if legacy {
		c.JSON(http.StatusBadRequest, body{Message: "legacy"}) // contract: mirrors the v1 envelope
		return
	}
	c.JSON(http.StatusOK, body{Message: "found"})
}
