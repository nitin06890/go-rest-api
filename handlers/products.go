package handlers

import (
	"context"
	"encoding/json"
	"io"

	"net/http"
	"net/url"

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

func findProducts(ctx context.Context, q url.Values, col dbiface.CollectionAPI) ([]Product, error) {
	var products []Product
	filter := make(map[string]interface{})
	for k, v := range q {
		filter[k] = v[0]
	}
	if filter["_id"] != nil {
		id, err := primitive.ObjectIDFromHex(filter["_id"].(string))
		if err != nil {
			log.Errorf("Unable to convert id to object id: %v", err)
			return nil, err
		}
		filter["_id"] = id
	}
	cursor, err := col.Find(ctx, bson.M(filter))
	if err != nil {
		log.Errorf("Unable to find the products: %v", err)
		return nil, err
	}
	err = cursor.All(ctx, &products)
	if err != nil {
		log.Errorf("Unable to decode the cursor to products: %v", err)
		return nil, err
	}
	return products, nil
}

// GetProducts returns all the products
func (h *ProductHandler) GetProducts(c echo.Context) error {
	products, err := findProducts(context.Background(), c.QueryParams(), h.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, products)
}

func findProduct(ctx context.Context, id string, col dbiface.CollectionAPI) (Product, error) {
	var product Product
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return product, err
	}
	filter := bson.M{"_id": docID}
	res := col.FindOne(ctx, filter)
	if err := res.Decode(&product); err != nil {
		log.Errorf("unable to decode to product :%v", err)
		return product, err
	}
	return product, nil
}

// GetProduct returns a product
func (h *ProductHandler) GetProduct(c echo.Context) error {
	product, err := findProduct(context.Background(), c.Param("id"), h.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, product)
}

func deleteProduct(ctx context.Context, id string, col dbiface.CollectionAPI) (int64, error) {
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return 0, err
	}
	filter := bson.M{"_id": docID}
	res, err := col.DeleteOne(ctx, filter)
	if err != nil {
		log.Errorf("unable to delete the product :%v", err)
		return 0, err
	}
	return res.DeletedCount, nil
}

// DeleteProduct deletes a product
func (h *ProductHandler) DeleteProduct(c echo.Context) error {
	delCount, err := deleteProduct(context.Background(), c.Param("id"), h.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, delCount)
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

func modifyProduct(ctx context.Context, id string, reqBody io.ReadCloser, collection dbiface.CollectionAPI) (Product, error) {
	var product Product
	// convert the id to ObjectID, if err return 400
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return product, err
	}
	filter := bson.M{"_id": docID}
	res := collection.FindOne(ctx, filter)
	if err := res.Decode(&product); err != nil {
		log.Errorf("unable to decode to product :%v", err)
		return product, err
	}

	//decode the request body to product, if err return 500
	if err := json.NewDecoder(reqBody).Decode(&product); err != nil {
		log.Errorf("unable to decode using reqbody : %v", err)
		return product, err
	}

	// validate the product, if err return 400
	if err := v.Struct(product); err != nil {
		log.Errorf("unable to validate the struct : %v", err)
		return product, err
	}

	// update the product, if err return 500
	_, err = collection.UpdateOne(ctx, filter, bson.M{"$set": product})
	if err != nil {
		log.Errorf("Unable to update the product : %v", err)
		return product, err
	}
	return product, nil
}

// UpdateProduct updates a product
func (h *ProductHandler) UpdateProduct(c echo.Context) error {
	product, err := modifyProduct(context.Background(), c.Param("id"), c.Request().Body, h.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, product)
}
