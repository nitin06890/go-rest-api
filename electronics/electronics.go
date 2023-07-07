package electronics

import (
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"gopkg.in/go-playground/validator.v9"
)

var e = echo.New()
var v = validator.New()

// Start starts the server
func Start() {
	port := os.Getenv("MY_APP_PORT")
	if port == "" {
		port = "8080"
	}

	e.GET("/products", getProducts)
	e.GET("/products/:id", getProductByID)
	e.DELETE("/products/:id", deleteProductByID)
	e.PUT("/products/:id", updateProductByID)
	e.POST("/products", createProduct)

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
