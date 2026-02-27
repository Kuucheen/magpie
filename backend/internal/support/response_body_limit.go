package support

import (
	"errors"
	"io"
)

var ErrResponseBodyTooLarge = errors.New("response body exceeds configured limit")

func ReadAllWithLimit(reader io.Reader, limitBytes int64) ([]byte, error) {
	if limitBytes <= 0 {
		return nil, ErrResponseBodyTooLarge
	}

	limited := io.LimitReader(reader, limitBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	if int64(len(body)) > limitBytes {
		return nil, ErrResponseBodyTooLarge
	}

	return body, nil
}
