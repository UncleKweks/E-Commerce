package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
)

// UseJSONFieldNames makes validation errors name fields the way clients
// send them ("first_name") instead of the Go name ("FirstName").
// Call it once at startup.
func UseJSONFieldNames() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				return ""
			}
			return name
		})
	}
}

// bindJSON decodes the request body into dst and runs its `binding` rules.
// On failure it writes the error response and returns false.
func bindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}

	var verrs validator.ValidationErrors
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &verrs):
		details := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			details[fe.Field()] = validationMessage(fe)
		}
		response.ValidationError(c, details)
	case errors.As(err, &tooLarge):
		response.Error(c, http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.")
	default:
		response.Error(c, http.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
	}
	return false
}

// validationMessage turns a validator rule into a readable sentence.
func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param() + " characters"
	case "max":
		return "must be at most " + fe.Param() + " characters"
	case "e164":
		return "must be in international format, e.g. +2348012345678"
	default:
		return "is invalid"
	}
}

// internalError logs the real error and sends the client a generic 500.
// The logger comes from the request context, so the log line carries the
// request ID (see middleware.RequestLogger). The client never sees err's text.
func internalError(c *gin.Context, err error) {
	zerolog.Ctx(c.Request.Context()).Error().Err(err).Str("route", c.FullPath()).Msg("request failed")
	response.InternalError(c)
}
