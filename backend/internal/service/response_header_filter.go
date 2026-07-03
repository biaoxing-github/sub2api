package service

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
)

const jsonContentTypeUTF8 = "application/json; charset=utf-8"

func compileResponseHeaderFilter(cfg *config.Config) *responseheaders.CompiledHeaderFilter {
	if cfg == nil {
		return nil
	}
	return responseheaders.CompileHeaderFilter(cfg.Security.ResponseHeaders)
}

// forceJSONContentType 在复制上游响应头之后覆盖聚合 JSON 响应类型。
func forceJSONContentType(header http.Header) {
	if header == nil {
		return
	}
	header.Set("Content-Type", jsonContentTypeUTF8)
}
