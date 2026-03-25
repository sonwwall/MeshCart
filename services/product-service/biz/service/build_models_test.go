package service

import (
	"testing"

	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"

	"github.com/bwmarrin/snowflake"
)

func TestBuildModelsForWrite_AllowsEmptySKUCode(t *testing.T) {
	svc := newTestProductService(t, repository.NewMySQLProductRepository(newProductTestDB(t), 0))

	productModel, skuModels, bizErr := svc.buildModelsForWrite(
		0,
		"Tee",
		"",
		1,
		"",
		"",
		ProductStatusOffline,
		[]*productpb.ProductSkuInput{
			{Title: "Blue XL", SalePrice: 100, Status: SKUStatusActive},
		},
		11,
		11,
	)
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if productModel == nil || len(skuModels) != 1 {
		t.Fatalf("unexpected models: product=%+v skus=%+v", productModel, skuModels)
	}
	if skuModels[0].SKUCode != "" {
		t.Fatalf("expected empty sku_code to be preserved, got %q", skuModels[0].SKUCode)
	}
}

func TestBuildModelsForWrite_RejectsDuplicateNonEmptySKUCode(t *testing.T) {
	svc := newTestProductService(t, repository.NewMySQLProductRepository(newProductTestDB(t), 0))

	_, _, bizErr := svc.buildModelsForWrite(
		0,
		"Tee",
		"",
		1,
		"",
		"",
		ProductStatusOffline,
		[]*productpb.ProductSkuInput{
			{SkuCode: "same", Title: "Blue XL", SalePrice: 100, Status: SKUStatusActive},
			{SkuCode: "same", Title: "White XL", SalePrice: 120, Status: SKUStatusActive},
		},
		11,
		11,
	)
	if bizErr == nil {
		t.Fatal("expected duplicate sku_code error")
	}
	if bizErr.Code == 0 {
		t.Fatalf("unexpected bizErr: %+v", bizErr)
	}
}

func newTestProductService(t *testing.T, repo repository.ProductRepository) *ProductService {
	t.Helper()
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	return NewProductService(repo, node, nil)
}
