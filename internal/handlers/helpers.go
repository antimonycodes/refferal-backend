package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, e := range validationErrors {
			switch e.Tag() {
			case "required":
				messages = append(messages, e.Field()+" is required")
			case "email":
				messages = append(messages, e.Field()+" must be a valid email")
			case "min":
				messages = append(messages, e.Field()+" must be at least "+e.Param()+" characters")
			case "oneof":
				messages = append(messages, e.Field()+" must be one of: "+e.Param())
			default:
				messages = append(messages, e.Field()+" is invalid")
			}
		}
		return strings.Join(messages, ", ")
	}
	return "validation failed"
}

func getQueryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	result, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return result
}
