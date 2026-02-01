package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getTestDatabase(t *testing.T) *mongo.Database {
	t.Helper()

	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://admin:admin@localhost:27017/auctions?authSource=admin"
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Skipf("Skipping test: could not connect to MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("Skipping test: MongoDB not available: %v", err)
	}

	// Use a dedicated test database to avoid conflicts
	return client.Database("auctions_test")
}

func TestAutoCloseAuction(t *testing.T) {
	// Setup: use a very short auction interval
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	db := getTestDatabase(t)
	collection := db.Collection("auctions")

	// Clean up before and after test
	ctx := context.Background()
	collection.Drop(ctx)
	defer collection.Drop(ctx)

	// Create a cancellable context for the auction closer goroutine
	closerCtx, closerCancel := context.WithCancel(ctx)
	defer closerCancel()

	// Create the repository (this starts the auto-closer goroutine)
	repo := NewAuctionRepository(closerCtx, db)

	// Insert an auction entity that was created 3 seconds ago (already expired with 2s interval)
	expiredAuction := &auction_entity.Auction{
		Id:          "test-auction-expired",
		ProductName: "Test Product",
		Category:    "Test Category",
		Description: "Test Description for the auction",
		Condition:   auction_entity.New,
		Status:      auction_entity.Active,
		Timestamp:   time.Now().Add(-3 * time.Second),
	}

	if err := repo.CreateAuction(ctx, expiredAuction); err != nil {
		t.Fatalf("Failed to create expired auction: %v", err)
	}

	// Insert a fresh auction that should NOT be closed yet
	activeAuction := &auction_entity.Auction{
		Id:          "test-auction-active",
		ProductName: "Active Product",
		Category:    "Test Category",
		Description: "This auction should stay open",
		Condition:   auction_entity.Used,
		Status:      auction_entity.Active,
		Timestamp:   time.Now(),
	}

	if err := repo.CreateAuction(ctx, activeAuction); err != nil {
		t.Fatalf("Failed to create active auction: %v", err)
	}

	// Wait for the auto-closer goroutine to run (check interval = interval/2 = 1s)
	time.Sleep(2 * time.Second)

	// Verify: the expired auction should now be Completed
	var expiredResult AuctionEntityMongo
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-expired"}).Decode(&expiredResult); err != nil {
		t.Fatalf("Failed to find expired auction: %v", err)
	}

	if expiredResult.Status != auction_entity.Completed {
		t.Errorf("Expected expired auction status to be Completed (%d), got %d",
			auction_entity.Completed, expiredResult.Status)
	}

	// Verify: the active auction should still be Active
	var activeResult AuctionEntityMongo
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-active"}).Decode(&activeResult); err != nil {
		t.Fatalf("Failed to find active auction: %v", err)
	}

	if activeResult.Status != auction_entity.Active {
		t.Errorf("Expected active auction status to be Active (%d), got %d",
			auction_entity.Active, activeResult.Status)
	}
}

func TestAutoCloseAuctionAfterExpiration(t *testing.T) {
	// Setup: use a very short auction interval
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	db := getTestDatabase(t)
	collection := db.Collection("auctions")

	// Clean up before and after test
	ctx := context.Background()
	collection.Drop(ctx)
	defer collection.Drop(ctx)

	// Create a cancellable context for the auction closer goroutine
	closerCtx, closerCancel := context.WithCancel(ctx)
	defer closerCancel()

	// Create the repository (this starts the auto-closer goroutine)
	repo := NewAuctionRepository(closerCtx, db)

	// Insert a fresh auction (Active and not expired)
	auctionEntity := &auction_entity.Auction{
		Id:          "test-auction-transition",
		ProductName: "Transition Product",
		Category:    "Electronics",
		Description: "This auction will expire during the test",
		Condition:   auction_entity.Refurbished,
		Status:      auction_entity.Active,
		Timestamp:   time.Now(),
	}

	if err := repo.CreateAuction(ctx, auctionEntity); err != nil {
		t.Fatalf("Failed to create auction: %v", err)
	}

	// Immediately after creation, auction should be Active
	var result AuctionEntityMongo
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-transition"}).Decode(&result); err != nil {
		t.Fatalf("Failed to find auction: %v", err)
	}
	if result.Status != auction_entity.Active {
		t.Errorf("Expected auction to be Active right after creation, got %d", result.Status)
	}

	// Wait for the auction interval to expire + buffer for the ticker to fire
	time.Sleep(4 * time.Second)

	// After the interval, auction should be Completed
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-transition"}).Decode(&result); err != nil {
		t.Fatalf("Failed to find auction after expiration: %v", err)
	}
	if result.Status != auction_entity.Completed {
		t.Errorf("Expected auction to be Completed after interval, got %d", result.Status)
	}
}
