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

func findProducts(ctx context.Context, q url.Values, col dbiface.CollectionAPI) ([]Product, *echo.HTTPError) {
	var products []Product
	filter := make(map[string]interface{})
	for k, v := range q {
		filter[k] = v[0]
	}
	if filter["_id"] != nil {
		id, err := primitive.ObjectIDFromHex(filter["_id"].(string))
		if err != nil {
			log.Errorf("Unable to convert id to object id: %v", err)
			return products, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to convert id to object id"})
		}
		filter["_id"] = id
	}
	cursor, err := col.Find(ctx, bson.M(filter))
	if err != nil {
		log.Errorf("Unable to find the products: %v", err)
		return products, echo.NewHTTPError(http.StatusNotFound, errorMessage{Message: "Unable to find the products"})
	}
	err = cursor.All(ctx, &products)
	if err != nil {
		log.Errorf("Unable to decode the cursor to products: %v", err)
		return products, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to decode the cursor to products"})
	}
	return products, nil
}

// GetProducts returns all the products
func (h *ProductHandler) GetProducts(c echo.Context) error {
	products, err := findProducts(context.Background(), c.QueryParams(), h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}
	return c.JSON(http.StatusOK, products)
}

func findProduct(ctx context.Context, id string, col dbiface.CollectionAPI) (Product, *echo.HTTPError) {
	var product Product
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return product, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to convert id to object id"})
	}
	filter := bson.M{"_id": docID}
	res := col.FindOne(ctx, filter)
	if err := res.Decode(&product); err != nil {
		log.Errorf("unable to decode to product :%v", err)
		return product, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to find the product"})
	}
	return product, nil
}

// GetProduct returns a product
func (h *ProductHandler) GetProduct(c echo.Context) error {
	product, err := findProduct(context.Background(), c.Param("id"), h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}
	return c.JSON(http.StatusOK, product)
}

func deleteProduct(ctx context.Context, id string, col dbiface.CollectionAPI) (int64, *echo.HTTPError) {
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to convert id to object id"})
	}
	filter := bson.M{"_id": docID}
	res, err := col.DeleteOne(ctx, filter)
	if err != nil {
		log.Errorf("unable to delete the product :%v", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to delete the product"})
	}
	return res.DeletedCount, nil
}

// DeleteProduct deletes a product
func (h *ProductHandler) DeleteProduct(c echo.Context) error {
	delCount, err := deleteProduct(context.Background(), c.Param("id"), h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}
	return c.JSON(http.StatusOK, delCount)
}

func insertProducts(ctx context.Context, products []Product, col dbiface.CollectionAPI) ([]interface{}, *echo.HTTPError) {
	var insertedIds []interface{}

	for _, product := range products {
		product.ID = primitive.NewObjectID()
		insertID, err := col.InsertOne(ctx, product)
		if err != nil {
			log.Errorf("Unable to insert to database: %v", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to insert to database"})
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
		return c.JSON(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to bind the request"})
	}
	for _, product := range products {
		if err := c.Validate(product); err != nil {
			log.Errorf("Unable to validate the product %+v: %v", product, err)
			return c.JSON(http.StatusBadRequest, errorMessage{Message: "Unable to validate the product"})
		}
	}
	IDs, err := insertProducts(context.Background(), products, h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}

	return c.JSON(http.StatusCreated, IDs)
}

func modifyProduct(ctx context.Context, id string, reqBody io.ReadCloser, collection dbiface.CollectionAPI) (Product, *echo.HTTPError) {
	var product Product
	// convert the id to ObjectID, if err return 400
	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Errorf("cannot convert to ObjectID :%v", err)
		return product, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to convert id to object id"})
	}
	filter := bson.M{"_id": docID}
	res := collection.FindOne(ctx, filter)
	if err := res.Decode(&product); err != nil {
		log.Errorf("unable to decode to product :%v", err)
		return product, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to find the product"})
	}

	//decode the request body to product, if err return 500
	if err := json.NewDecoder(reqBody).Decode(&product); err != nil {
		log.Errorf("unable to decode using reqbody : %v", err)
		return product, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to decode the request body"})
	}

	// validate the product, if err return 400
	if err := v.Struct(product); err != nil {
		log.Errorf("unable to validate the struct : %v", err)
		return product, echo.NewHTTPError((http.StatusBadRequest), errorMessage{Message: "Unable to validate the product"})
	}

	// update the product, if err return 500
	_, err = collection.UpdateOne(ctx, filter, bson.M{"$set": product})
	if err != nil {
		log.Errorf("Unable to update the product : %v", err)
		return product, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to update the product"})
	}
	return product, nil
}

// UpdateProduct updates a product
func (h *ProductHandler) UpdateProduct(c echo.Context) error {
	product, err := modifyProduct(context.Background(), c.Param("id"), c.Request().Body, h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}
	return c.JSON(http.StatusOK, product)
}
