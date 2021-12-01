package gin_contrib_lz4

import (
	"fmt"
	lz4 "github.com/pierrec/lz4/v4"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type lz4Handler struct {
	*Options
	zPool sync.Pool
}

func newLz4Handler(options ...Option) *lz4Handler {
	var lz4Pool sync.Pool
	lz4Pool.New = func() interface{} {
		return lz4.NewWriter(nil)
	}
	handler := &lz4Handler{
		Options: DefaultOptions,
		zPool:   lz4Pool,
	}
	for _, setter := range options {
		setter(handler.Options)
	}
	return handler
}

func (l *lz4Handler) Handle(c *gin.Context) {
	if fn := l.DecompressFn; fn != nil && c.Request.Header.Get("Content-Encoding") == "lz4" {
		fn(c)
	}

	if !l.shouldCompress(c.Request) {
		return
	}

	lz := l.zPool.Get().(*lz4.Writer)
	defer l.zPool.Put(lz)
	defer lz.Reset(ioutil.Discard)
	lz.Reset(c.Writer)

	c.Header("Content-Encoding", "lz4")
	c.Header("Vary", "Accept-Encoding")
	c.Writer = &lz4Writer{c.Writer, lz}
	defer func() {
		lz.Close()
		c.Header("Content-Length", fmt.Sprint(c.Writer.Size()))
	}()
	c.Next()
}

func (l *lz4Handler) shouldCompress(req *http.Request) bool {
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "lz4") ||
		strings.Contains(req.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(req.Header.Get("Content-Type"), "text/event-stream") {

		return false
	}

	extension := filepath.Ext(req.URL.Path)
	if l.ExcludedExtensions.Contains(extension) {
		return false
	}

	if l.ExcludedPaths.Contains(req.URL.Path) {
		return false
	}
	if l.ExcludedPathesRegexs.Contains(req.URL.Path) {
		return false
	}

	return true
}
