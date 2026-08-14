package session

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type Target struct {
	New   bool
	Index int
}

func ResolveTarget(r *http.Request) (Target, error) {
	queryValue := strings.TrimSpace(r.URL.Query().Get("authuser"))
	headerValue := strings.TrimSpace(r.Header.Get("X-Auth-User-Index"))
	if queryValue != "" && headerValue != "" && queryValue != headerValue {
		return Target{}, errors.New("authuser query and header do not match")
	}
	value := queryValue
	if value == "" {
		value = headerValue
	}
	if value == "" {
		value = "0"
	}
	if value == "new" {
		return Target{New: true, Index: -1}, nil
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return Target{}, errors.New("authuser must be new or a non-negative integer")
	}
	return Target{Index: index}, nil
}
