package auction

import (
	"context"
	"fmt"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func createDatabaseContainer(ctx context.Context, t *testing.T) *mongodb.MongoDBContainer {
	t.Helper()

	mongodbContainer, err := mongodb.Run(ctx, "mongo:latest")
	require.NoError(t, err)

	return mongodbContainer
}

func getTestDatabase(ctx context.Context, t *testing.T) (*mongo.Client, *mongo.Database, *mongodb.MongoDBContainer) {
	t.Helper()

	mongoContainer := createDatabaseContainer(ctx, t)

	mongoURL, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Skipf("Skipping test: could not connect to MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("Skipping test: MongoDB not available: %v", err)
	}

	return client, client.Database("auctions_test"), mongoContainer
}

func TestAutoCloseAuction(t *testing.T) {
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	ctx := context.Background()

	client, db, container := getTestDatabase(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	collectionName := fmt.Sprintf("auctions_test_%d", time.Now().UnixNano())
	collection := db.Collection(collectionName)
	collection.Drop(ctx)
	defer collection.Drop(ctx)

	closerCtx, closerCancel := context.WithCancel(ctx)
	defer func() {
		closerCancel()                     // Cancela o contexto
		time.Sleep(100 * time.Millisecond) // Dá tempo para a goroutine terminar
	}()

	repo := NewAuctionRepositoryWithCollection(closerCtx, db, collectionName)

	// Criar leilão expirado
	expiredAuction := &auction_entity.Auction{
		Id:          "test-auction-expired",
		ProductName: "Test Product",
		Category:    "Test Category",
		Description: "Test Description for the auction",
		Condition:   auction_entity.New,
		Status:      auction_entity.Active,
		Timestamp:   time.Now().Add(-3 * time.Second), // 3s no passado
	}

	if err := repo.CreateAuction(ctx, expiredAuction); err != nil {
		t.Fatalf("Failed to create expired auction: %v", err)
	}

	// Aguardar um pouco antes de criar o leilão ativo
	time.Sleep(500 * time.Millisecond)

	// Criar leilão ativo DEPOIS
	activeAuction := &auction_entity.Auction{
		Id:          "test-auction-active",
		ProductName: "Active Product",
		Category:    "Test Category",
		Description: "This auction should stay open",
		Condition:   auction_entity.Used,
		Status:      auction_entity.Active,
		Timestamp:   time.Now().Add(time.Second * 6),
	}

	if err := repo.CreateAuction(ctx, activeAuction); err != nil {
		t.Fatalf("Failed to create active auction: %v", err)
	}

	// Aguardar a verificação
	time.Sleep(2500 * time.Millisecond) // Um pouco mais que 2s

	var expiredResult AuctionEntityMongo
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-expired"}).Decode(&expiredResult); err != nil {
		t.Fatalf("Failed to find expired auction: %v", err)
	}

	if expiredResult.Status != auction_entity.Completed {
		t.Errorf("Expected expired auction status to be Completed (%d), got %d",
			auction_entity.Completed, expiredResult.Status)
	}

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
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	ctx := context.Background()

	client, db, container := getTestDatabase(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	collectionName := fmt.Sprintf("auctions_test_%d", time.Now().UnixNano())
	collection := db.Collection(collectionName)
	collection.Drop(ctx)
	defer collection.Drop(ctx)

	closerCtx, closerCancel := context.WithCancel(ctx)
	defer func() {
		closerCancel()                     // Cancela o contexto
		time.Sleep(100 * time.Millisecond) // Dá tempo para a goroutine terminar
	}()

	repo := NewAuctionRepositoryWithCollection(closerCtx, db, collectionName)

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

	var result AuctionEntityMongo
	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-transition"}).Decode(&result); err != nil {
		t.Fatalf("Failed to find auction: %v", err)
	}
	if result.Status != auction_entity.Active {
		t.Errorf("Expected auction to be Active right after creation, got %d", result.Status)
	}

	time.Sleep(4 * time.Second)

	if err := collection.FindOne(ctx, bson.M{"_id": "test-auction-transition"}).Decode(&result); err != nil {
		t.Fatalf("Failed to find auction after expiration: %v", err)
	}
	if result.Status != auction_entity.Completed {
		t.Errorf("Expected auction to be Completed after interval, got %d", result.Status)
	}
}
