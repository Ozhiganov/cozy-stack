// Package data provide simple CRUD operation on couchdb doc
package data

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cozy/cozy-stack/couchdb"
	"github.com/cozy/cozy-stack/web/jsonapi"
	"github.com/cozy/cozy-stack/web/middlewares"
	"github.com/labstack/echo"
)

func validDoctype(next echo.HandlerFunc) echo.HandlerFunc {
	// TODO extends me to verificate characters allowed in db name.
	return func(c echo.Context) error {
		doctype := c.Param("doctype")
		if doctype == "" {
			return jsonapi.NewError(http.StatusBadRequest, "Invalid doctype '%s'", doctype)
		}
		c.Set("doctype", doctype)
		return next(c)
	}
}

// GetDoc get a doc by its type and id
func getDoc(c echo.Context) error {
	instance := middlewares.GetInstance(c)
	doctype := c.Get("doctype").(string)
	docid := c.Param("docid")

	if err := CheckReadable(c, doctype); err != nil {
		return err
	}

	var out couchdb.JSONDoc
	err := couchdb.GetDoc(instance, doctype, docid, &out)
	if err != nil {
		return err
	}

	out.Type = doctype
	return c.JSON(http.StatusOK, out.ToMapWithType())
}

// CreateDoc create doc from the json passed as body
func createDoc(c echo.Context) error {
	doctype := c.Get("doctype").(string)
	instance := middlewares.GetInstance(c)

	var doc = couchdb.JSONDoc{Type: doctype}
	if err := c.Bind(&doc.M); err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	if err := CheckWritable(c, doctype); err != nil {
		return err
	}

	if doc.ID() != "" {
		return jsonapi.NewError(http.StatusBadRequest,
			"Cannot create a document with _id")
	}

	err := couchdb.CreateDoc(instance, doc)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, echo.Map{
		"ok":   true,
		"id":   doc.ID(),
		"rev":  doc.Rev(),
		"type": doc.DocType(),
		"data": doc.ToMapWithType(),
	})
}

func updateDoc(c echo.Context) error {
	instance := middlewares.GetInstance(c)

	var doc couchdb.JSONDoc
	if err := c.Bind(&doc); err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	doc.Type = c.Param("doctype")

	if err := CheckWritable(c, doc.Type); err != nil {
		return err
	}

	if (doc.ID() == "") != (doc.Rev() == "") {
		return jsonapi.NewError(http.StatusBadRequest,
			"You must either provide an _id and _rev in document (update) or neither (create with  fixed id).")
	}

	if doc.ID() != "" && doc.ID() != c.Param("docid") {
		return jsonapi.NewError(http.StatusBadRequest, "document _id doesnt match url")
	}

	var err error
	if doc.ID() == "" {
		doc.SetID(c.Param("docid"))
		err = couchdb.CreateNamedDoc(instance, doc)
	} else {
		err = couchdb.UpdateDoc(instance, doc)
	}

	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"ok":   true,
		"id":   doc.ID(),
		"rev":  doc.Rev(),
		"type": doc.DocType(),
		"data": doc.ToMapWithType(),
	})
}

func deleteDoc(c echo.Context) error {
	instance := middlewares.GetInstance(c)
	doctype := c.Get("doctype").(string)
	docid := c.Param("docid")
	revHeader := c.Request().Header.Get("If-Match")
	revQuery := c.QueryParam("rev")
	rev := ""

	if revHeader != "" && revQuery != "" && revQuery != revHeader {
		return jsonapi.NewError(http.StatusBadRequest,
			"If-Match Header and rev query parameters mismatch")
	} else if revHeader != "" {
		rev = revHeader
	} else if revQuery != "" {
		rev = revQuery
	} else {
		return jsonapi.NewError(http.StatusBadRequest, "delete without revision")
	}

	if err := CheckWritable(c, doctype); err != nil {
		return err
	}

	tombrev, err := couchdb.Delete(instance, doctype, docid, rev)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{
		"ok":      true,
		"id":      docid,
		"rev":     tombrev,
		"type":    doctype,
		"deleted": true,
	})

}

func defineIndex(c echo.Context) error {
	instance := middlewares.GetInstance(c)
	doctype := c.Get("doctype").(string)
	var definitionRequest map[string]interface{}

	if err := c.Bind(&definitionRequest); err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	if err := CheckWritable(c, doctype); err != nil {
		return err
	}

	result, err := couchdb.DefineIndexRaw(instance, doctype, &definitionRequest)
	if couchdb.IsNoDatabaseError(err) {
		if err = couchdb.CreateDB(instance, doctype); err == nil {
			result, err = couchdb.DefineIndexRaw(instance, doctype, &definitionRequest)
		}
	}
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, result)
}

func findDocuments(c echo.Context) error {
	instance := middlewares.GetInstance(c)
	doctype := c.Get("doctype").(string)
	var findRequest map[string]interface{}

	if err := c.Bind(&findRequest); err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	if err := CheckReadable(c, doctype); err != nil {
		return err
	}

	var results []couchdb.JSONDoc
	err := couchdb.FindDocsRaw(instance, doctype, &findRequest, &results)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, echo.Map{"docs": results})
}

var allowedChangesParams = map[string]bool{
	"feed":  true,
	"style": true,
	"since": true,
	"limit": true,
}

func changesFeed(c echo.Context) error {
	instance := middlewares.GetInstance(c)

	// Drop a clear error for parameters not supported by stack
	for key := range c.QueryParams() {
		if !allowedChangesParams[key] {
			err := fmt.Errorf("Unsuported query parameter '%s'", key)
			return jsonapi.NewError(http.StatusBadRequest, err)
		}
	}

	feed, err := couchdb.ValidChangesMode(c.QueryParam("feed"))
	if err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	feedStyle, err := couchdb.ValidChangesStyle(c.QueryParam("style"))
	if err != nil {
		return jsonapi.NewError(http.StatusBadRequest, err)
	}

	var limitString = c.QueryParam("limit")
	var limit = 0
	if limitString != "" {
		if limit, err = strconv.Atoi(limitString); err != nil {
			err = fmt.Errorf("Invalid limit value '%s'", err.Error())
			return jsonapi.NewError(http.StatusBadRequest, err)
		}
	}

	results, err := couchdb.GetChanges(instance, &couchdb.ChangesRequest{
		DocType: c.Get("doctype").(string),
		Feed:    feed,
		Style:   feedStyle,
		Since:   c.QueryParam("since"),
		Limit:   limit,
	})

	return c.JSON(http.StatusOK, results)
}

// Routes sets the routing for the status service
func Routes(router *echo.Group) {
	router.Use(validDoctype)
	router.GET("/:doctype/_changes", changesFeed)
	// POST=GET see http://docs.couchdb.org/en/2.0.0/api/database/changes.html#post--db-_changes)
	router.POST("/:doctype/_changes", changesFeed)
	router.GET("/:doctype/:docid", getDoc)
	router.PUT("/:doctype/:docid", updateDoc)
	router.DELETE("/:doctype/:docid", deleteDoc)
	router.POST("/:doctype/", createDoc)
	router.POST("/:doctype/_index", defineIndex)
	router.POST("/:doctype/_find", findDocuments)
	// router.DELETE("/:doctype/:docid", DeleteDoc)
}
