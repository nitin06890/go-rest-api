package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
	"gopkg.in/go-playground/validator.v9"
)

// PrductValidator echo validator for product
type PrductValidator struct {
	validator *validator.Validate
}

// Validate validates product request body
func (p *PrductValidator) Validate(i interface{}) error {
	return p.validator.Struct(i)
}

func main() {
	port := os.Getenv("MY_APP_PORT")
	if port == "" {
		port = "8080"
	}

	e := echo.New()
	v := validator.New()
	products := []map[int]string{{1: "mobiles"}, {2: "laptops"}, {3: "tablets"}}

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, Shelby!")
	})
	e.GET("/products/:id", func(c echo.Context) error {
		var product map[int]string
		for _, p := range products {
			for k := range p {
				pID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					return err
				}
				if pID == k {
					product = p
				}
			}
		}
		if product == nil {
			return c.JSON(http.StatusNotFound, "Product not found")
		}
		return c.JSON(http.StatusOK, product)
	})

	e.POST("/products", func(c echo.Context) error {
		type body struct {
			Name string `json:"product_name" validate:"required,min=4"`
		}
		var reqBody body
		e.Validator = &PrductValidator{validator: v}
		if err := c.Bind(&reqBody); err != nil {
			return err
		}
		if err := c.Validate(reqBody); err != nil {
			return err
		}

		products = append(products, map[int]string{len(products) + 1: reqBody.Name})
		return c.JSON(http.StatusCreated, "Product created")
	})

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}
