package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	internal "github.com/dcermak/container-layer-sizes/pkg"

	"github.com/stretchr/testify/suite"
)

var s *internal.SQLiteBackend

type BackendTestSuite struct {
	suite.Suite
	s       *internal.SQLiteBackend
	handler http.HandlerFunc
	rr      *httptest.ResponseRecorder
}

func (b *BackendTestSuite) SetupTest() {
	b.s = s
	b.handler = http.HandlerFunc(backend(s))
	b.rr = httptest.NewRecorder()
}

func (b *BackendTestSuite) TestSimpleGet404() {
	req, err := http.NewRequest("GET", "/", nil)
	b.Nilf(err, "Failed to create request: %s", err)

	q := req.URL.Query()
	q.Add("id", "1")
	req.URL.RawQuery = q.Encode()

	b.handler.ServeHTTP(b.rr, req)
	b.Equalf(http.StatusNotFound, b.rr.Code, "requesting an invalid id must result in a 404, body: %s", b.rr.Body)
}

func (b *BackendTestSuite) TestSimpleGetByName404() {
	req, err := http.NewRequest("GET", "/", nil)
	b.Nilf(err, "Failed to create request: %s", err)

	q := req.URL.Query()
	q.Add("name", "foobar")
	req.URL.RawQuery = q.Encode()

	b.handler.ServeHTTP(b.rr, req)
	b.Equalf(http.StatusNotFound, b.rr.Code, "requesting an invalid name must result in a 404, body: %s", b.rr.Body)
}

func TestBackendTestSuite(t *testing.T) {
	suite.Run(t, new(BackendTestSuite))
}

func TestMain(m *testing.M) {

	file, err := ioutil.TempFile("", "testDb.*.sqlite3")
	if err != nil {
		panic(err)
	}
	defer os.Remove(file.Name())

	if s, err = internal.CreateSQLiteBackend(file.Name()); err != nil {
		panic(err)
	}

	code := m.Run()

	if err = s.Destroy(); err != nil {
		panic(err)
	}

	// os.Exit bypasses defer
	os.Remove(file.Name())
	os.Exit(code)
}
