package handlers

import (
	"context"

	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/nitin06890/go-rest-api/dbiface"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/go-playground/validator.v9"
)

var (
	v = validator.New()
)

// Product describes an electronic product
type Product struct {
	ID          primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"product_name" bson:"product_name" validate:"required,max=10"`
	Price       int                `json:"price" bson:"price" validate:"required,max=1000"`
	Currency    string             `json:"currency" bson:"currency" validate:"required,len=3"`
	Discount    int                `json:"discount" bson:"discount"`
	Vendor      string             `json:"vendor" bson:"vendor" validate:"required"`
	Accessories []string           `json:"accessories,omitempty" bson:"accessories,omitempty"`
	IsEssential bool               `json:"is_essential" bson:"is_essential"`
}

// ProductHandler handles product related requests
type ProductHandler struct {
	Col dbiface.CollectionAPI
}

// ProductValidator validates the product
type ProductValidator struct {
	validator *validator.Validate
}

// Validate validates the product
func (p *ProductValidator) Validate(i interface{}) error {
	return p.validator.Struct(i)
}

func findProducts(ctx context.Context, col dbiface.CollectionAPI) ([]Product, error) {
	var products []Product
	cursor, err := col.Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Unable to find the products: %v", err)
	}
	err = cursor.All(ctx, &products)
	if err != nil {
		log.Errorf("Unable to decode the cursor to products: %v", err)
	}
	return products, nil
}

// GetProducts returns all the products
func (h *ProductHandler) GetProducts(c echo.Context) error {
	products, err := findProducts(context.Background(), h.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, products)
}

func insertProducts(ctx context.Context, products []Product, col dbiface.CollectionAPI) ([]interface{}, error) {
	var insertedIds []interface{}

	for _, product := range products {
		product.ID = primitive.NewObjectID()
		insertID, err := col.InsertOne(ctx, product)
		if err != nil {
			log.Errorf("Unable to insert: %v", err)
			return nil, err
		}
		insertedIds = append(insertedIds, insertID.InsertedID)
	}
	return insertedIds, nil
}

func (h *ProductHandler) CreateProducts(c echo.Context) error {
	var products []Product

	c.Echo().Validator = &ProductValidator{validator: v}
	if err := c.Bind(&products); err != nil {
		log.Errorf("Unable to bind the request: %v", err)
		return err
	}
	for _, product := range products {
		if err := c.Validate(product); err != nil {
			log.Errorf("Unable to validate the product %+v: %v", product, err)
			return err
		}
	}
	IDs, err := insertProducts(context.Background(), products, h.Col)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, IDs)
}
