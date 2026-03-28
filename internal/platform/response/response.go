package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSON sends a JSON response with the given status code and data.
func JSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// BadRequest sends a 400 Bad Request response with an error message.
func BadRequest(c *gin.Context, message string, err error) {
	response := gin.H{
		"error":   "bad_request",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	c.JSON(http.StatusBadRequest, response)
}

// Unauthorized sends a 401 Unauthorized response with an error message.
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error":   "unauthorized",
		"message": message,
	})
}

// NotFound sends a 404 Not Found response with an error message.
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "not_found",
		"message": message,
	})
}

// Error sends a 500 Internal Server Error response with an error message.
func Error(c *gin.Context, message string, err error) {
	response := gin.H{
		"error":   "internal_error",
		"message": message,
	}
	if err != nil {
		response["details"] = err.Error()
	}
	c.JSON(http.StatusInternalServerError, response)
}

// Created sends a 201 Created response with the given data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

// OK sends a 200 OK response with the given data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}