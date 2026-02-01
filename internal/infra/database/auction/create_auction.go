package auction

import (
	"context"
	"fmt"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
}

func NewAuctionRepository(ctx context.Context, database *mongo.Database) *AuctionRepository {
	repo := &AuctionRepository{
		Collection:      database.Collection("auctions"),
		auctionInterval: getAuctionInterval(),
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

// startAuctionCloser runs a background goroutine that periodically checks for
// expired auctions (Active auctions whose creation time + AUCTION_INTERVAL has passed)
// and updates their status to Completed in MongoDB.
func (ar *AuctionRepository) startAuctionCloser(ctx context.Context) {
	// Check at a reasonable frequency — half the auction interval or at least every second
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

// closeExpiredAuctions finds all Active auctions whose timestamp + auctionInterval
// is in the past and updates their status to Completed.
func (ar *AuctionRepository) closeExpiredAuctions(ctx context.Context) {
	expirationThreshold := time.Now().Add(-ar.auctionInterval).Unix()

	filter := bson.M{
		"status":    auction_entity.Active,
		"timestamp": bson.M{"$lte": expirationThreshold},
	}

	update := bson.M{
		"$set": bson.M{"status": auction_entity.Completed},
	}

	result, err := ar.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		logger.Error("Error trying to close expired auctions", err)
		return
	}

	if result.ModifiedCount > 0 {
		logger.Info(fmt.Sprintf("Successfully closed %d expired auction(s)", result.ModifiedCount))
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
