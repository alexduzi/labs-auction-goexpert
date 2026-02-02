package auction

import (
	"context"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection      *mongo.Collection
	auctionInterval time.Duration
	mutex           *sync.Mutex
}

func NewAuctionRepository(ctx context.Context, database *mongo.Database) *AuctionRepository {
	return NewAuctionRepositoryWithCollection(ctx, database, "auctions")
}

func NewAuctionRepositoryWithCollection(ctx context.Context, database *mongo.Database, collectionName string) *AuctionRepository {
	repo := &AuctionRepository{
		Collection:      database.Collection(collectionName),
		auctionInterval: getAuctionInterval(),
		mutex:           &sync.Mutex{},
	}

	go repo.startAuctionCloser(ctx)

	return repo
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	return nil
}

func (ar *AuctionRepository) startAuctionCloser(ctx context.Context) {
	// Verifica com mais frequência do que o intervalo de expiração
	checkInterval := ar.auctionInterval / 2
	if checkInterval < time.Second {
		checkInterval = time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ar.closeExpiredAuctions(ctx)
		}
	}
}

func (ar *AuctionRepository) closeExpiredAuctions(ctx context.Context) {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()

	expirationThreshold := time.Now().Add(-ar.auctionInterval).Unix()

	filter := bson.M{
		"status": auction_entity.Active,
		"timestamp": bson.M{
			"$lt": expirationThreshold,
		},
	}

	update := bson.M{
		"$set": bson.M{
			"status": auction_entity.Completed,
		},
	}

	logger.Info("Checking for expired auctions",
		zap.Int64("threshold", expirationThreshold),
		zap.Int64("now", time.Now().Unix()))

	result, err := ar.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		logger.Error("Error trying to close expired auctions", err)
		return
	}

	if result.ModifiedCount > 0 {
		logger.Info("Successfully closed expired auctions",
			zap.Int64("count", result.ModifiedCount))
	} else {
		logger.Info("No expired auctions found")
	}
}

func getAuctionInterval() time.Duration {
	auctionInterval := os.Getenv("AUCTION_INTERVAL")
	duration, err := time.ParseDuration(auctionInterval)
	if err != nil {
		return time.Minute * 5
	}

	return duration
}
