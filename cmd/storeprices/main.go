// Command storeprices creates the consumable Google Play products that cart
// orders are paid with — one per whole Rp 1.000 from Rp 1.000 to the cart
// maximum (arunika.cart.t1000 ... arunika.cart.t500000) — and records them
// in store_products. It is safe to run again: products already in
// store_products are skipped and a product Google already has is recorded
// without being created twice.
//
//	go run ./cmd/storeprices -dry-run
//	go run ./cmd/storeprices
//
// Credentials come from the environment like the API server
// (GOOGLE_PLAY_SERVICE_ACCOUNT_JSON, ANDROID_PACKAGE_NAME, DB_*), which the
// deploy supplies from the secret store.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"arunika_backend/config"
	"arunika_backend/models"
	"arunika_backend/services"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "list the missing products without creating anything")
	max := flag.Int64("max", services.CartMaxTotalIdr, "highest total in rupiah")
	flag.Parse()

	db, err := gorm.Open(postgres.Open(config.LoadDBConfig().DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	var existing []models.StoreProduct
	if err := db.Where("store = ?", models.StoreGoogle).Find(&existing).Error; err != nil {
		log.Fatalf("load store_products: %v", err)
	}
	have := map[int64]bool{}
	for _, p := range existing {
		have[p.PriceIdr] = true
	}

	var missing []int64
	for total := services.CartPriceStepIdr; total <= *max; total += services.CartPriceStepIdr {
		if !have[total] {
			missing = append(missing, total)
		}
	}
	fmt.Printf("%d products recorded, %d missing\n", len(existing), len(missing))
	if *dryRun {
		for _, total := range missing {
			fmt.Println(services.StoreProductSKU(total))
		}
		return
	}

	play := services.NewGooglePlayVerifier()
	created, recorded := 0, 0
	for _, total := range missing {
		sku := services.StoreProductSKU(total)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := play.CreateCartProduct(ctx, sku, total)
		cancel()
		switch {
		case err == nil:
			created++
		case errors.Is(err, services.ErrPlayProductExists):
		default:
			log.Fatalf("create %s: %v (created %d so far; re-run to continue)", sku, err, created)
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.StoreProduct{
			Store: models.StoreGoogle, PlayProductID: sku, PriceIdr: total, Active: true,
		}).Error; err != nil {
			log.Fatalf("record %s: %v", sku, err)
		}
		recorded++
		// Stay well inside the Android Publisher API quota.
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("created %d in Google Play, recorded %d in store_products\n", created, recorded)
}
