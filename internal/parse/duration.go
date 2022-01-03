package parse

import (
	"errors"
	"fmt"
	"time"
)

var originTime = createOriginTime()

func createOriginTime() time.Time {
	o, _ := time.Parse("15:04:05.000", "00:00:00.000")
	return o
}

func TimeAsDuration(format, t string) (time.Duration, error) {
	parsed, err := time.Parse(format, t)
	if err != nil {
		return 0, err
	}
	return parsed.Sub(originTime), nil
}

func Duration(t string) (time.Duration, error) {
	var err error
	var parsed time.Duration

	// Attempt to parse HH:MM:SS
	parsed, err = TimeAsDuration("15:04:05", t)
	if err == nil {
		return parsed, nil
	}

	// Attempt to parse MM:SS
	parsed, err = TimeAsDuration("04:05", t)
	if err == nil {
		return parsed, nil
	}
	parsed, err = TimeAsDuration("4:05", t)
	if err == nil {
		return parsed, nil
	}

	// Attempt to parse seconds
	parsed, err = time.ParseDuration(fmt.Sprintf("%ss", t))
	if err == nil {
		return parsed, nil
	}

	return 0, errors.New("failed to parse trim time")
}

