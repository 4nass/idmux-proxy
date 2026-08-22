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
	if r == nil || r.URL == nil {
		return Target{}, errors.New("request URL is required")
	}

	queryValue, err := singleTargetValue(r.URL.Query()["authuser"], "authuser query")
	if err != nil {
		return Target{}, err
	}
	headerValue, err := singleTargetValue(r.Header.Values("X-Auth-User-Index"), "authuser header")
	if err != nil {
		return Target{}, err
	}
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

func singleTargetValue(values []string, source string) (string, error) {
	if len(values) > 1 {
		return "", errors.New(source + " must appear at most once")
	}
	if len(values) == 0 {
		return "", nil
	}
	return strings.TrimSpace(values[0]), nil
}
