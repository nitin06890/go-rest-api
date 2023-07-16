package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/nitin06890/go-rest-api/dbiface"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/go-playground/validator.v9"
)

// User represents a user
type User struct {
	Email    string `json:"username" bson:"username" validate:"required,email"`
	Password string `json:"password" bson:"password" validate:"required,min=8,max=300"`
}

// UsersHandler handles user related requests
type UsersHandler struct {
	Col dbiface.CollectionAPI
}

type userValidator struct {
	validator *validator.Validate
}

func (u *userValidator) Validate(i interface{}) error {
	return u.validator.Struct(i)
}

// CreateUser creates a user
func (u *UsersHandler) CreateUser(c echo.Context) error {
	var user User
	c.Echo().Validator = &userValidator{validator: v}
	if err := c.Bind(&user); err != nil {
		log.Errorf("Unable to bind user: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
	}
	if err := c.Validate(user); err != nil {
		log.Errorf("Unable to validate user: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
	}
	insertedUserID, err := insertUser(context.Background(), user, u.Col)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, insertedUserID)
}

func insertUser(ctx context.Context, user User, col dbiface.CollectionAPI) (interface{}, *echo.HTTPError) {
	var newUser User
	// Check if user already exists
	res := col.FindOne(ctx, bson.M{"username": user.Email})
	err := res.Decode(&newUser)
	if err == nil && err != mongo.ErrNoDocuments {
		log.Errorf("Unable to decode retrieved user: %v", err)
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, "Unable to decode retrieved user")
	}
	// If user already exists, return error
	if newUser.Email != "" {
		log.Errorf("User by %s already exists", user.Email)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "User already exists")
	}
	// If user doesn't exist, insert user
	insertRes, err := col.InsertOne(ctx, user)
	if err != nil {
		log.Errorf("Unable to insert user: %+v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Unable to insert user")
	}
	return insertRes.InsertedID, nil
}
