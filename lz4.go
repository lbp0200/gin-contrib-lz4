package gin_contrib_lz4

import (
	"github.com/gin-gonic/gin"
	"github.com/pierrec/lz4/v4"
)

func Lz4(options ...Option) gin.HandlerFunc {
	return newLz4Handler(options...).Handle
}

type lz4Writer struct {
	gin.ResponseWriter
	writer *lz4.Writer
}

func (lw *lz4Writer) WriteString(s string) (int, error) {
	lw.Header().Del("Content-Length")
	return lw.writer.Write([]byte(s))
}

func (lw *lz4Writer) Write(data []byte) (int, error) {
	lw.Header().Del("Content-Length")
	return lw.writer.Write(data)
}

// Fix: https://github.com/mholt/caddy/issues/38
func (lw *lz4Writer) WriteHeader(code int) {
	lw.Header().Del("Content-Length")
	lw.ResponseWriter.WriteHeader(code)
}
