package util

import (
    "net/http"
    "strconv"
    "time"
)

func ParseGithubRateLimitReset(header http.Header) (time.Time, error) {
    resetStr := header.Get("X-RateLimit-Reset")
    if resetStr == "" {
        return time.Time{}, nil
    }
    unixSec, err := strconv.ParseInt(resetStr, 10, 64)
    if err != nil {
        return time.Time{}, err
    }
    return time.Unix(unixSec, 0), nil
}
