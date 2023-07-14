package main

import (
	"context"
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/labstack/gommon/random"
	"github.com/nitin06890/go-rest-api/config"
	"github.com/nitin06890/go-rest-api/handlers"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// CorrelationID is the header key for correlation ID
	CorrelationID = "X-Correlation-ID"
)

var (
	// c   *mongo.Client
	db  *mongo.Database
	col *mongo.Collection
	cfg config.Properties
)

func init() {
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("Configuration cannot be read: %v", err)
	}
	ctx := context.Background()

	connectURI := fmt.Sprintf("mongodb://%s:%s", cfg.DBHost, cfg.DBPort)
	c, err := mongo.Connect(ctx, options.Client().ApplyURI(connectURI))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	db = c.Database(cfg.DBName)
	col = db.Collection(cfg.CollectionName)
}

func main() {
	e := echo.New()
	e.Logger.SetLevel(log.ERROR)
	h := &handlers.ProductHandler{Col: col}
	e.Pre(middleware.RemoveTrailingSlash())
	e.Pre(addCorrelationID)
	e.GET("/products", h.GetProducts)
	e.GET("/products/:id", h.GetProduct)
	e.DELETE("/products/:id", h.DeleteProduct)
	e.POST("/products", h.CreateProducts, middleware.BodyLimit("1M"))
	e.PUT("/products/:id", h.UpdateProduct, middleware.BodyLimit("1M"))

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
