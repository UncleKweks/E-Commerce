package handlers

import (
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
)

// skuPattern allows letters, digits and dashes, e.g. "TSHIRT-RED-M".
var skuPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// SetupValidator configures request validation. Call it once at startup.
//
//   - Errors name fields the way clients send them ("first_name") instead of
//     the Go name ("FirstName"). Query parameters use their `form` name.
//   - Adds two rules of our own: `sku` and `notblank` (rejects "   ").
func SetupValidator() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		tag := f.Tag.Get("json")
		if tag == "" {
			tag = f.Tag.Get("form")
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			return ""
		}
		return name
	})
	_ = v.RegisterValidation("sku", func(fl validator.FieldLevel) bool {
		return skuPattern.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	})
}

// bindJSON decodes the request body into dst and runs its `binding` rules.
// On failure it writes the error response and returns false.
func bindJSON(c *gin.Context, dst any) bool {
	return handleBindError(c, c.ShouldBindJSON(dst), "invalid_json", "The request body must be valid JSON.")
}

// bindQuery reads URL query parameters (?page=2&sort=price_asc) into dst
// and runs its `binding` rules. On failure it writes the error response
// and returns false.
func bindQuery(c *gin.Context, dst any) bool {
	return handleBindError(c, c.ShouldBindQuery(dst), "invalid_query", "The query parameters are invalid.")
}

func handleBindError(c *gin.Context, err error, code, message string) bool {
	if err == nil {
		return true
	}

	var verrs validator.ValidationErrors
	var tooLarge *http.MaxBytesError
	var numErr *strconv.NumError
	switch {
	case errors.As(err, &verrs):
		details := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			details[fe.Field()] = validationMessage(fe)
		}
		response.ValidationError(c, details)
	case errors.As(err, &tooLarge):
		response.Error(c, http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.")
	case errors.As(err, &numErr):
		// e.g. ?page=abc or "price_cents": "ten"
		response.Error(c, http.StatusBadRequest, code, "A number field contains something that isn't a whole number.")
	default:
		response.Error(c, http.StatusBadRequest, code, message)
	}
	return false
}

// validationMessage turns a validator rule into a readable sentence.
func validationMessage(fe validator.FieldError) string {
	// min/max mean "length" for text and "value" for numbers.
	isText := fe.Kind() == reflect.String
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min", "gte":
		if isText {
			return "must be at least " + fe.Param() + " characters"
		}
		return "must be at least " + fe.Param()
	case "max", "lte":
		if isText {
			return "must be at most " + fe.Param() + " characters"
		}
		return "must be at most " + fe.Param()
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "e164":
		return "must be in international format, e.g. +2348012345678"
	case "sku":
		return "may only contain letters, numbers and dashes"
	case "notblank":
		return "must not be blank"
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

// idParam reads a positive numeric ID from the URL, e.g. /products/42.
// On failure it writes a 404 (a non-numeric ID can't match anything) and
// returns false.
func idParam(c *gin.Context) (uint, bool) {
	return uintParam(c, "id")
}

// uintParam is idParam for a URL parameter with another name.
func uintParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusNotFound, "not_found", "Not found.")
		return 0, false
	}
	return uint(id), true
}
