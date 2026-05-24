package service

import "io"

type apiKeyPassthroughEmptyReader struct{}

func (r *apiKeyPassthroughEmptyReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}
