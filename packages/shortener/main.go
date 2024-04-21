package main

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type urlMap struct {
	Id        string `json:"id" db:"id"`
	Url       string `json:"url" db:"url"`
	Shortened string `json:"shortened" db:"shortened"`
}

type shortenRequestBody struct {
	Url string `json:"url"`
}

func getErrorCodeFromDBError(err error) int {
	errString := err.Error()
	if strings.Contains(errString, "no rows") {
		return http.StatusNotFound
	}

	return http.StatusInternalServerError
}

/*
Gets a single url by id
*/
func getUrl(c *gin.Context, db *sqlx.DB) {
	id, exists := c.Params.Get("id")
	if !exists {
		c.JSON(http.StatusBadRequest, "ID missing")
		return
	}
	var url urlMap
	err := db.Get(&url, "select * from url where shortened=$1;", id)
	if err != nil {
		log.Println(err)
		c.JSON(getErrorCodeFromDBError(err), "An error occurred")
		return
	}
	c.Redirect(http.StatusFound, url.Url)
}

/*
Gets all saved urls
*/
func getUrls(c *gin.Context, db *sqlx.DB) {
	var urls []urlMap
	err := db.Select(&urls, "select * from url;")
	if err != nil {
		log.Println(err)
		c.JSON(getErrorCodeFromDBError(err), "An error occurred")
		return
	}
	c.JSON(http.StatusOK, urls)
}

/*
Lookup path then redirect to associated website
*/
func redirect(c *gin.Context, db *sqlx.DB) {
	path, exists := c.Params.Get("path")
	if !exists {
		c.JSON(http.StatusBadRequest, "Path missing")
	}
	var url urlMap
	err := db.Get(&url, "select * from url where id=$1;", path)
	if err != nil {
		log.Println(err)
		c.JSON(getErrorCodeFromDBError(err), "An error occurred")
		return
	}
	c.Redirect(http.StatusFound, url.Url)
}

/*
Takes a request body that contains a url, creates a unique path for it and saves it to the db
*/
func shorten(c *gin.Context, db *sqlx.DB) {
	var requestBody shortenRequestBody
	err := c.BindJSON(&requestBody)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, "An error occurred")
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, "An error occurred")
		return
	}
	hash := fmt.Sprintf("%x", md5.Sum([]byte(id.String())))
	path := hash[:len(hash)-20]
	_, err = db.Exec("insert into url (url, shortened) values ($1, $2);", requestBody.Url, path)
	if err != nil {
		log.Println(err)
		c.JSON(getErrorCodeFromDBError(err), "An error occurred")
		return
	}

	var newUrlMap urlMap
	// assign struct vals incase db query fails
	newUrlMap.Url = requestBody.Url
	newUrlMap.Shortened = path
	_ = db.Get(&newUrlMap, "select * from url where shortened=$1", path)
	c.JSON(http.StatusCreated, newUrlMap)
}

func createRouter(db *sqlx.DB) *gin.Engine {
	router := gin.Default()

	router.GET("/url", func(c *gin.Context) {
		getUrls(c, db)
	})
	router.POST("/url", func(c *gin.Context) {
		shorten(c, db)
	})
	router.GET("/url/:id", func(c *gin.Context) {
		getUrl(c, db)
	})
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/:path", func(c *gin.Context) {
		redirect(c, db)
	})

	return router
}

func main() {
	connStr := "postgresql://postgres:password@localhost:5432/urls?sslmode=disable"
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	router := createRouter(db)
	router.Run("localhost:8080")
}
