package gin_contrib_lz4

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pierrec/lz4/v4"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testResponse        = "Lz4 Test Response "
	testReverseResponse = "Lz4 Test Reverse Response "
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type rServer struct{}

func (s *rServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprint(rw, testReverseResponse)
}

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func newCloseNotifyingRecorder() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func (c *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return c.closed
}

func newServer() *gin.Engine {
	// init reverse proxy server
	rServer := httptest.NewServer(new(rServer))
	target, _ := url.Parse(rServer.URL)
	rp := httputil.NewSingleHostReverseProxy(target)

	router := gin.New()
	router.Use(Lz4())
	router.GET("/", func(c *gin.Context) {
		c.Header("Content-Length", strconv.Itoa(len(testResponse)))
		c.String(200, testResponse)
	})
	router.Any("/reverse", func(c *gin.Context) {
		rp.ServeHTTP(c.Writer, c.Request)
	})
	return router
}

func TestLz4(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("Accept-Encoding", "lz4")

	w := httptest.NewRecorder()
	r := newServer()
	r.ServeHTTP(w, req)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Header().Get("Content-Encoding"), "lz4")
	assert.Equal(t, w.Header().Get("Vary"), "Accept-Encoding")
	assert.NotEqual(t, w.Header().Get("Content-Length"), "0")
	assert.NotEqual(t, w.Body.Len(), 19)
	assert.Equal(t, fmt.Sprint(w.Body.Len()), w.Header().Get("Content-Length"))

	lr := lz4.NewReader(w.Body)
	body, _ := ioutil.ReadAll(lr)
	assert.Equal(t, string(body), testResponse)
}

func TestLz4PNG(t *testing.T) {
	req, _ := http.NewRequest("GET", "/image.png", nil)
	req.Header.Add("Accept-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4())
	router.GET("/image.png", func(c *gin.Context) {
		c.String(200, "this is a PNG!")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Header().Get("Content-Encoding"), "")
	assert.Equal(t, w.Header().Get("Vary"), "")
	assert.Equal(t, w.Body.String(), "this is a PNG!")
}

func TestExcludedExtensions(t *testing.T) {
	req, _ := http.NewRequest("GET", "/index.html", nil)
	req.Header.Add("Accept-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4(WithExcludedExtensions([]string{".html"})))
	router.GET("/index.html", func(c *gin.Context) {
		c.String(200, "this is a HTML!")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "", w.Header().Get("Vary"))
	assert.Equal(t, "this is a HTML!", w.Body.String())
	assert.Equal(t, "", w.Header().Get("Content-Length"))
}

func TestExcludedPaths(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/books", nil)
	req.Header.Add("Accept-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4(WithExcludedPaths([]string{"/api/"})))
	router.GET("/api/books", func(c *gin.Context) {
		c.String(200, "this is books!")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "", w.Header().Get("Vary"))
	assert.Equal(t, "this is books!", w.Body.String())
	assert.Equal(t, "", w.Header().Get("Content-Length"))
}

func TestNoLz4(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)

	w := httptest.NewRecorder()
	r := newServer()
	r.ServeHTTP(w, req)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Header().Get("Content-Encoding"), "")
	assert.Equal(t, w.Header().Get("Content-Length"), "18")
	assert.Equal(t, w.Body.String(), testResponse)
}

func TestLz4WithReverseProxy(t *testing.T) {
	req, _ := http.NewRequest("GET", "/reverse", nil)
	req.Header.Add("Accept-Encoding", "lz4")

	w := newCloseNotifyingRecorder()
	r := newServer()
	r.ServeHTTP(w, req)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, w.Header().Get("Content-Encoding"), "lz4")
	assert.Equal(t, w.Header().Get("Vary"), "Accept-Encoding")
	assert.NotEqual(t, w.Header().Get("Content-Length"), "0")
	assert.NotEqual(t, w.Body.Len(), 19)
	assert.Equal(t, fmt.Sprint(w.Body.Len()), w.Header().Get("Content-Length"))

	lr := lz4.NewReader(w.Body)
	body, _ := ioutil.ReadAll(lr)
	assert.Equal(t, string(body), testReverseResponse)
}

func TestDecompressLz4(t *testing.T) {
	buf := &bytes.Buffer{}
	lw := lz4.NewWriter(buf)
	if _, err := lw.Write([]byte(testResponse)); err != nil {
		t.Fatal(err)
	}
	lw.Close()
	req, _ := http.NewRequest("POST", "/", buf)
	req.Header.Add("Content-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4(WithDecompressFn(DefaultDecompressHandle)))
	router.POST("/", func(c *gin.Context) {
		if v := c.Request.Header.Get("Content-Encoding"); v != "" {
			t.Errorf("unexpected `Content-Encoding`: %s header", v)
		}
		if v := c.Request.Header.Get("Content-Length"); v != "" {
			t.Errorf("unexpected `Content-Length`: %s header", v)
		}
		data, err := c.GetRawData()
		if err != nil {
			t.Fatal(err)
		}
		c.Data(200, "text/plain", data)
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "", w.Header().Get("Vary"))
	assert.Equal(t, testResponse, w.Body.String())
	assert.Equal(t, "", w.Header().Get("Content-Length"))
}

func TestDecompressLz4WithEmptyBody(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", nil)
	req.Header.Add("Content-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4(WithDecompressFn(DefaultDecompressHandle)))
	router.POST("/", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Header().Get("Content-Encoding"))
	assert.Equal(t, "", w.Header().Get("Vary"))
	assert.Equal(t, "ok", w.Body.String())
	assert.Equal(t, "", w.Header().Get("Content-Length"))
}

func TestDecompressLz4WithIncorrectData(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", bytes.NewReader([]byte(testResponse)))
	req.Header.Add("Content-Encoding", "lz4")

	router := gin.New()
	router.Use(Lz4(WithDecompressFn(DefaultDecompressHandle)))
	router.POST("/", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
