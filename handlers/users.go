package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/nitin06890/go-rest-api/config"
	"github.com/nitin06890/go-rest-api/dbiface"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user
type User struct {
	Email    string `json:"username" bson:"username" validate:"required,email"`
	Password string `json:"password,omitempty" bson:"password" validate:"required,min=8,max=300"`
	IsAdmin  bool   `json:"isadmin,omitempty" bson:"isadmin"`
}

// UsersHandler handles user related requests
type UsersHandler struct {
	Col dbiface.CollectionAPI
}

type errorMessage struct {
	Message string `json:"message"`
}

var (
	prop config.Properties
)

// CreateUser creates a user
func (h *UsersHandler) CreateUser(c echo.Context) error {
	var user User
	c.Echo().Validator = &userValidator{validator: v}
	if err := c.Bind(&user); err != nil {
		log.Errorf("Unable to bind user: %v", err)
		return c.JSON(http.StatusBadRequest, errorMessage{Message: "Unable to bind user"})
	}
	if err := c.Validate(user); err != nil {
		log.Errorf("Unable to validate user: %v", err)
		return c.JSON(http.StatusBadRequest, errorMessage{Message: "Unable to validate user"})
	}
	insertedUserID, err := insertUser(context.Background(), user, h.Col)
	if err != nil {
		return c.JSON(err.Code, err.Message)
	}
	token, er := user.generateToken()
	if er != nil {
		log.Errorf("Unable to generate token: %v", er)
		return c.JSON(http.StatusInternalServerError, errorMessage{Message: "Unable to generate token"})
	}
	c.Response().Header().Set("x-auth-token", token)
	return c.JSON(http.StatusCreated, insertedUserID)
}

func insertUser(ctx context.Context, user User, col dbiface.CollectionAPI) (interface{}, *echo.HTTPError) {
	var newUser User
	// Check if user already exists
	res := col.FindOne(ctx, bson.M{"username": user.Email})
	err := res.Decode(&newUser)
	if err == nil && err != mongo.ErrNoDocuments {
		log.Errorf("Unable to decode retrieved user: %v", err)
		return nil, echo.NewHTTPError(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to decode retrieved user"})
	}
	// If user already exists, return error
	if newUser.Email != "" {
		log.Errorf("User by %s already exists", user.Email)
		return nil, echo.NewHTTPError(http.StatusBadRequest, errorMessage{Message: "User already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("Unable to hash password: %+v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to hash password"})
	}
	user.Password = string(hashedPassword)

	// If user doesn't exist, insert user
	_, err = col.InsertOne(ctx, user)
	if err != nil {
		log.Errorf("Unable to insert user: %+v", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, errorMessage{Message: "Unable to insert user"})
	}
	return User{Email: user.Email}, nil
}

func (h *UsersHandler) AuthnUser(ctx echo.Context) error {
	var user User
	ctx.Echo().Validator = &userValidator{validator: v}
	if err := ctx.Bind(&user); err != nil {
		log.Errorf("Unable to bind user: %v", err)
		return ctx.JSON(http.StatusUnprocessableEntity, errorMessage{Message: "Unable to bind user"})
	}
	if err := ctx.Validate(user); err != nil {
		log.Errorf("Unable to validate user: %v", err)
		return ctx.JSON(http.StatusBadRequest, errorMessage{Message: "Unable to validate user"})
	}
	authenticatedUser, httpError := authenticateUser(context.Background(), user, h.Col)
	if httpError != nil {
		log.Errorf("Unable to authenticate user: %v", httpError)
		return ctx.JSON(httpError.Code, httpError.Message)
	}
	token, err := user.generateToken()
	if err != nil {
		log.Errorf("Unable to generate token: %v", err)
		return ctx.JSON(http.StatusInternalServerError, errorMessage{Message: "Unable to generate token"})
	}
	ctx.Response().Header().Set("x-auth-token", token)
	return ctx.JSON(http.StatusOK, User{Email: authenticatedUser.Email})
}

func authenticateUser(ctx context.Context, reqUser User, col dbiface.CollectionAPI) (User, *echo.HTTPError) {
	var storedUser User
	res := col.FindOne(ctx, bson.M{"username": reqUser.Email})
	err := res.Decode(&storedUser)
	if err == nil && err != mongo.ErrNoDocuments {
		log.Errorf("User by %s doesn't exist", reqUser.Email)
		return User{}, echo.NewHTTPError(http.StatusBadRequest, errorMessage{Message: "User doesn't exist"})
	}
	// Validate the password
	err = bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(reqUser.Password))
	if err != nil {
		log.Errorf("Invalid password: %v", err)
		return User{}, echo.NewHTTPError(http.StatusUnauthorized, errorMessage{Message: "Invalid password"})
	}
	return User{Email: storedUser.Email}, nil
}

func (u User) generateToken() (string, error) {
	if err := cleanenv.ReadEnv(&prop); err != nil {
		log.Fatalf("Unable to read configuration: %v", err)
	}
	claims := jwt.MapClaims{}
	claims["authorized"] = u.IsAdmin
	claims["user_id"] = u.Email
	claims["exp"] = time.Now().Add(time.Minute * 15).Unix()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := at.SignedString([]byte(prop.JwtTokenSecret))
	if err != nil {
		log.Errorf("Unable to generate the token: %v", err)
		return "", err
	}
	return token, nil
}
