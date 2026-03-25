package service

import (
	"context"
	"testing"

	productpb "meshcart/kitex_gen/meshcart/product"
	"meshcart/services/product-service/biz/repository"
	productredis "meshcart/services/product-service/dal/redis"

	"github.com/bwmarrin/snowflake"
)

type stubCache struct {
	products          map[int64]*productpb.Product
	productLists      map[string]stubProductList
	skus              map[int64]*productpb.ProductSku
	deletedProductIDs []int64
	deletedSKUIDs     []int64
	deletedListCount  int
}

type stubProductList struct {
	items []*productpb.ProductListItem
	total int64
}

var _ productredis.Cache = (*stubCache)(nil)

func (s *stubCache) GetProducts(_ context.Context, productIDs []int64) (map[int64]*productpb.Product, error) {
	result := make(map[int64]*productpb.Product, len(productIDs))
	for _, productID := range productIDs {
		if product, ok := s.products[productID]; ok {
			result[productID] = product
		}
	}
	return result, nil
}

func (s *stubCache) SetProducts(_ context.Context, products []*productpb.Product) error {
	if s.products == nil {
		s.products = make(map[int64]*productpb.Product, len(products))
	}
	for _, product := range products {
		s.products[product.GetId()] = product
	}
	return nil
}

func (s *stubCache) DeleteProducts(_ context.Context, productIDs []int64) error {
	s.deletedProductIDs = append(s.deletedProductIDs, productIDs...)
	for _, productID := range productIDs {
		delete(s.products, productID)
	}
	return nil
}

func (s *stubCache) GetProductList(_ context.Context, cacheKey string) ([]*productpb.ProductListItem, int64, bool, error) {
	if item, ok := s.productLists[cacheKey]; ok {
		return item.items, item.total, true, nil
	}
	return nil, 0, false, nil
}

func (s *stubCache) SetProductList(_ context.Context, cacheKey string, items []*productpb.ProductListItem, total int64) error {
	if s.productLists == nil {
		s.productLists = make(map[string]stubProductList)
	}
	s.productLists[cacheKey] = stubProductList{items: items, total: total}
	return nil
}

func (s *stubCache) DeleteProductLists(_ context.Context) error {
	s.deletedListCount++
	s.productLists = make(map[string]stubProductList)
	return nil
}

func (s *stubCache) GetSKUs(_ context.Context, skuIDs []int64) (map[int64]*productpb.ProductSku, error) {
	result := make(map[int64]*productpb.ProductSku, len(skuIDs))
	for _, skuID := range skuIDs {
		if sku, ok := s.skus[skuID]; ok {
			result[skuID] = sku
		}
	}
	return result, nil
}

func (s *stubCache) SetSKUs(_ context.Context, skus []*productpb.ProductSku) error {
	if s.skus == nil {
		s.skus = make(map[int64]*productpb.ProductSku, len(skus))
	}
	for _, sku := range skus {
		s.skus[sku.GetId()] = sku
	}
	return nil
}

func (s *stubCache) DeleteSKUs(_ context.Context, skuIDs []int64) error {
	s.deletedSKUIDs = append(s.deletedSKUIDs, skuIDs...)
	for _, skuID := range skuIDs {
		delete(s.skus, skuID)
	}
	return nil
}

func newCachedProductService(t *testing.T, repo repository.ProductRepository, cache productredis.Cache) *ProductService {
	t.Helper()
	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("new snowflake node: %v", err)
	}
	return NewProductService(repo, node, cache)
}

func TestProductService_BatchGetProducts_UsesCache(t *testing.T) {
	repo := repository.NewMySQLProductRepository(newProductTestDB(t), 0)
	cache := &stubCache{
		products: map[int64]*productpb.Product{
			1001: {Id: 1001, Title: "Cached Tee", Status: ProductStatusOnline},
		},
	}
	svc := newCachedProductService(t, repo, cache)

	products, bizErr := svc.BatchGetProducts(context.Background(), []int64{1001})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if len(products) != 1 || products[0].GetTitle() != "Cached Tee" {
		t.Fatalf("unexpected products: %+v", products)
	}
}

func TestProductService_BatchGetSKU_UsesCache(t *testing.T) {
	repo := repository.NewMySQLProductRepository(newProductTestDB(t), 0)
	cache := &stubCache{
		skus: map[int64]*productpb.ProductSku{
			2001: {Id: 2001, SpuId: 1001, Title: "Cached Blue XL", Status: SKUStatusActive, SalePrice: 1999},
		},
	}
	svc := newCachedProductService(t, repo, cache)

	skus, bizErr := svc.BatchGetSKU(context.Background(), []int64{2001})
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if len(skus) != 1 || skus[0].GetTitle() != "Cached Blue XL" {
		t.Fatalf("unexpected skus: %+v", skus)
	}
}

func TestProductService_GetProductDetail_UsesCache(t *testing.T) {
	repo := repository.NewMySQLProductRepository(newProductTestDB(t), 0)
	cache := &stubCache{
		products: map[int64]*productpb.Product{
			1001: {Id: 1001, Title: "Cached Detail Tee", Status: ProductStatusOnline},
		},
	}
	svc := newCachedProductService(t, repo, cache)

	product, bizErr := svc.GetProductDetail(context.Background(), 1001)
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if product.GetTitle() != "Cached Detail Tee" {
		t.Fatalf("unexpected product: %+v", product)
	}
}

func TestProductService_ListProducts_UsesCache(t *testing.T) {
	repo := repository.NewMySQLProductRepository(newProductTestDB(t), 0)
	req := &productpb.ListProductsRequest{Page: 1, PageSize: 20}
	cacheKey := productListCacheKey(req, 1, 20)
	cache := &stubCache{
		productLists: map[string]stubProductList{
			cacheKey: {
				items: []*productpb.ProductListItem{{
					Id:    1001,
					Title: "Cached List Tee",
					Status: ProductStatusOnline,
				}},
				total: 1,
			},
		},
	}
	svc := newCachedProductService(t, repo, cache)

	items, total, bizErr := svc.ListProducts(context.Background(), req)
	if bizErr != nil {
		t.Fatalf("expected nil bizErr, got %+v", bizErr)
	}
	if total != 1 || len(items) != 1 || items[0].GetTitle() != "Cached List Tee" {
		t.Fatalf("unexpected result: total=%d items=%+v", total, items)
	}
}

func TestProductService_UpdateProduct_InvalidatesCache(t *testing.T) {
	repo := repository.NewMySQLProductRepository(newProductTestDB(t), 0)
	cache := &stubCache{
		productLists: map[string]stubProductList{
			"page=1:size=20:keyword=:status_set=false:status=0:category_set=false:category=0:creator_set=false:creator=0": {
				items: []*productpb.ProductListItem{{Id: 1, Title: "stale"}},
				total: 1,
			},
		},
	}
	svc := newCachedProductService(t, repo, cache)

	productID, _, bizErr := svc.CreateProduct(context.Background(), &productpb.CreateProductRequest{
		Title:       "MeshCart Tee",
		CategoryId:  1,
		Status:      ProductStatusOnline,
		CreatorId:   88,
		Description: "desc",
		Skus: []*productpb.ProductSkuInput{{
			Title:       "Blue XL",
			SalePrice:   1999,
			MarketPrice: 2999,
			Status:      SKUStatusActive,
		}},
	})
	if bizErr != nil {
		t.Fatalf("create product failed: %+v", bizErr)
	}

	updatedSKUs, bizErr := svc.UpdateProduct(context.Background(), &productpb.UpdateProductRequest{
		ProductId:    productID,
		Title:        "MeshCart Tee 2",
		CategoryId:   1,
		Status:       ProductStatusOnline,
		OperatorId:   88,
		Description:  "desc2",
		Skus: []*productpb.ProductSkuInput{{
			Title:       "Blue XL 2",
			SalePrice:   2099,
			MarketPrice: 3099,
			Status:      SKUStatusActive,
		}},
	})
	if bizErr == nil && len(updatedSKUs) == 0 {
		t.Fatalf("expected update product to return skus")
	}
	if len(cache.deletedProductIDs) == 0 || cache.deletedProductIDs[0] != productID {
		t.Fatalf("expected product cache invalidation, got %+v", cache.deletedProductIDs)
	}
	if cache.deletedListCount == 0 {
		t.Fatalf("expected product list cache invalidation")
	}
}
