package router

import (
	"net/http"
	"strings"
)

type CerebrasRouter struct {
	cerebrasHosts []string
}

func NewCerebrasRouter(fallback http.Handler) *CerebrasRouter {
	return &CerebrasRouter{
		cerebrasHosts: []string{
			"api.cerebras.ai",
			"inference.cerebras.ai",
		},
	}
}

func (cr *CerebrasRouter) IsCerebrasRequest(req *http.Request) bool {
	host := req.Host
	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	for _, cerebrasHost := range cr.cerebrasHosts {
		if host == cerebrasHost {
			return true
		}
	}

	return false
}
