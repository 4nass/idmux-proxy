package session

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Target struct {
	New   bool
	Index int
}

func ResolveTarget(r *http.Request) (Target, error) {
	return ResolveTargetWithDefault(r, Target{Index: 0})
}

// ResolveTargetWithDefault uses missing only when both selectors are absent.
func ResolveTargetWithDefault(r *http.Request, missing Target) (Target, error) {
	if r == nil || r.URL == nil {
		return Target{}, errors.New("request URL is required")
	}

	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return Target{}, errors.New("invalid routing query")
	}
	queryValue, err := singleTargetValue(query["authuser"], "authuser query")
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
		return missing, nil
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
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", errors.New(source + " is empty")
	}
	return value, nil
}
