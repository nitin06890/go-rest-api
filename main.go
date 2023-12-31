package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ilyakaznacheev/cleanenv"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/labstack/gommon/random"
	"github.com/nitin06890/go-rest-api/config"
	"github.com/nitin06890/go-rest-api/handlers"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// CorrelationID is the header key for correlation ID
	CorrelationID = "X-Correlation-ID"
)

var (
	c        *mongo.Client
	db       *mongo.Database
	prodCol  *mongo.Collection
	usersCol *mongo.Collection
	cfg      config.Properties
	err      error
)

func init() {
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("Configuration cannot be read: %v", err)
	}
	ctx := context.Background()

	connectURI := fmt.Sprintf("mongodb://%s:%s", cfg.DBHost, cfg.DBPort)
	c, err = mongo.Connect(ctx, options.Client().ApplyURI(connectURI))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	db = c.Database(cfg.DBName)
	prodCol = db.Collection(cfg.ProductCollection)
	usersCol = db.Collection(cfg.UsersCollection)

	isUserIndexUnique := true
	indexmodel := mongo.IndexModel{
		Keys: bson.D{{Key: "username", Value: 1}},
		Options: &options.IndexOptions{
			Unique: &isUserIndexUnique,
		},
	}

	if _, err := usersCol.Indexes().CreateOne(ctx, indexmodel); err != nil {
		log.Fatalf("Unable to create index: %v", err)
	}
}

func main() {
	e := echo.New()
	e.Logger.SetLevel(log.ERROR)
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(addCorrelationID)
	jwtMiddleware := echojwt.WithConfig(echojwt.Config{
		SigningKey:  []byte(cfg.JwtTokenSecret),
		TokenLookup: "header:x-auth-token",
	})
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339} ${remote_ip} ${header:X-Correlation-ID} ${host} ${method} ${uri} ${user_agent} ` +
			`${status} ${error} ${latency_human}` + "\n",
	}))
	h := &handlers.ProductHandler{Col: prodCol}
	uh := &handlers.UsersHandler{Col: usersCol}
	e.GET("/products", h.GetProducts)
	e.GET("/products/:id", h.GetProduct)
	e.DELETE("/products/:id", h.DeleteProduct, jwtMiddleware, adminMiddleware)
	e.POST("/products", h.CreateProducts, middleware.BodyLimit("1M"), jwtMiddleware)
	e.PUT("/products/:id", h.UpdateProduct, middleware.BodyLimit("1M"), jwtMiddleware)

	e.POST("/users", uh.CreateUser, middleware.BodyLimit("1M"))
	e.POST("/auth", uh.AuthnUser)
	e.Logger.Info("Listening on %s:%s ", cfg.Host, cfg.Port)
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)))
}

func addCorrelationID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var newID string
		id := c.Request().Header.Get(CorrelationID)
		if id == "" {
			newID = random.String(12)
		} else {
			newID = id
		}
		c.Request().Header.Set(CorrelationID, newID)
		c.Response().Header().Set(CorrelationID, newID)
		return next(c)
	}
}

func adminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Request().Header.Get("x-auth-token")
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
			return []byte(cfg.JwtTokenSecret), nil
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to parse token")
		}
		if !claims["authorized"].(bool) {
			return echo.NewHTTPError(http.StatusForbidden, "Not authorized")
		}
		return next(c)
	}
}
