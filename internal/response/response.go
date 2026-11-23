// Package response writes every API reply in one consistent JSON shape:
//
//	success: {"data": ...}
//	failure: {"error": {"code": "...", "message": "...", "details": {...}}}
//
// The HTTP status code says whether it worked; the body says what happened.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type errorBody struct {
	// Code is stable and machine-readable, for frontend code to switch on.
	Code string `json:"code"`
	// Message is for humans and may change wording.
	Message string `json:"message"`
	// Details holds per-field problems, e.g. {"email": "must be a valid email"}.
	Details map[string]string `json:"details,omitempty"`
}

// OK replies 200 with data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// PageMeta describes one page of a longer list, so clients can show
// "page 2 of 5" and know when to stop asking for more.
type PageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// NewPageMeta works out TotalPages from the total row count.
func NewPageMeta(page, pageSize int, total int64) PageMeta {
	pages := int((total + int64(pageSize) - 1) / int64(pageSize)) // round up
	return PageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: pages}
}

// Paged replies 200 with one page of a list: {"data": [...], "meta": {...}}.
func Paged(c *gin.Context, data any, meta PageMeta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

// Created replies 201 with the newly created resource.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

// NoContent replies 204 with an empty body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error replies with status and a safe, client-facing message.
// Never pass err.Error() from a library or the database as message: it can
// reveal table names, SQL or file paths to attackers. Log those instead.
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": errorBody{Code: code, Message: message}})
}

// ValidationError replies 422 with a message for each invalid field.
func ValidationError(c *gin.Context, details map[string]string) {
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": errorBody{
		Code:    "validation_failed",
		Message: "Some fields are invalid.",
		Details: details,
	}})
}

// InternalError replies 500 without any detail. The real error must be
// logged by the caller, tagged with the request ID so it can be found.
func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "internal_error",
		"Something went wrong on our side. Please try again.")
}
