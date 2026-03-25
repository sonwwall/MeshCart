package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	productpb "meshcart/kitex_gen/meshcart/product"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Address     string
	Password    string
	DB          int
	KeyPrefix   string
	TTL         time.Duration
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

type Cache interface {
	GetProducts(ctx context.Context, productIDs []int64) (map[int64]*productpb.Product, error)
	SetProducts(ctx context.Context, products []*productpb.Product) error
	DeleteProducts(ctx context.Context, productIDs []int64) error
	GetProductList(ctx context.Context, cacheKey string) ([]*productpb.ProductListItem, int64, bool, error)
	SetProductList(ctx context.Context, cacheKey string, items []*productpb.ProductListItem, total int64) error
	DeleteProductLists(ctx context.Context) error
	GetSKUs(ctx context.Context, skuIDs []int64) (map[int64]*productpb.ProductSku, error)
	SetSKUs(ctx context.Context, skus []*productpb.ProductSku) error
	DeleteSKUs(ctx context.Context, skuIDs []int64) error
}

type ProductCache struct {
	client    *goredis.Client
	keyPrefix string
	ttl       time.Duration
}

type cachedProductList struct {
	Items []*productpb.ProductListItem `json:"items"`
	Total int64                        `json:"total"`
}

func New(cfg Config) (*ProductCache, error) {
	cli := goredis.NewClient(&goredis.Options{
		Addr:        cfg.Address,
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), maxPositive(cfg.DialTimeout, 500*time.Millisecond))
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, err
	}

	return &ProductCache{
		client:    cli,
		keyPrefix: cfg.KeyPrefix,
		ttl:       maxPositive(cfg.TTL, 60*time.Second),
	}, nil
}

func (c *ProductCache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *ProductCache) GetProducts(ctx context.Context, productIDs []int64) (map[int64]*productpb.Product, error) {
	keys := make([]string, 0, len(productIDs))
	for _, productID := range productIDs {
		keys = append(keys, c.productKey(productID))
	}
	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*productpb.Product, len(values))
	for idx, raw := range values {
		if raw == nil {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		var product productpb.Product
		if err := json.Unmarshal([]byte(text), &product); err != nil {
			continue
		}
		result[productIDs[idx]] = &product
	}
	return result, nil
}

func (c *ProductCache) SetProducts(ctx context.Context, products []*productpb.Product) error {
	pipe := c.client.Pipeline()
	for _, product := range products {
		if product == nil || product.GetId() <= 0 {
			continue
		}
		payload, err := json.Marshal(product)
		if err != nil {
			return err
		}
		pipe.Set(ctx, c.productKey(product.GetId()), payload, c.ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *ProductCache) DeleteProducts(ctx context.Context, productIDs []int64) error {
	keys := make([]string, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID > 0 {
			keys = append(keys, c.productKey(productID))
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *ProductCache) GetProductList(ctx context.Context, cacheKey string) ([]*productpb.ProductListItem, int64, bool, error) {
	if cacheKey == "" {
		return nil, 0, false, nil
	}
	value, err := c.client.Get(ctx, c.productListKey(cacheKey)).Result()
	if err != nil {
		if err == goredis.Nil {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}

	var payload cachedProductList
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return nil, 0, false, nil
	}
	return payload.Items, payload.Total, true, nil
}

func (c *ProductCache) SetProductList(ctx context.Context, cacheKey string, items []*productpb.ProductListItem, total int64) error {
	if cacheKey == "" {
		return nil
	}
	payload, err := json.Marshal(cachedProductList{Items: items, Total: total})
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.productListKey(cacheKey), payload, c.ttl).Err()
}

func (c *ProductCache) DeleteProductLists(ctx context.Context) error {
	pattern := c.productListKey("*")
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			return nil
		}
	}
}

func (c *ProductCache) GetSKUs(ctx context.Context, skuIDs []int64) (map[int64]*productpb.ProductSku, error) {
	keys := make([]string, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		keys = append(keys, c.skuKey(skuID))
	}
	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*productpb.ProductSku, len(values))
	for idx, raw := range values {
		if raw == nil {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		var sku productpb.ProductSku
		if err := json.Unmarshal([]byte(text), &sku); err != nil {
			continue
		}
		result[skuIDs[idx]] = &sku
	}
	return result, nil
}

func (c *ProductCache) SetSKUs(ctx context.Context, skus []*productpb.ProductSku) error {
	pipe := c.client.Pipeline()
	for _, sku := range skus {
		if sku == nil || sku.GetId() <= 0 {
			continue
		}
		payload, err := json.Marshal(sku)
		if err != nil {
			return err
		}
		pipe.Set(ctx, c.skuKey(sku.GetId()), payload, c.ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *ProductCache) DeleteSKUs(ctx context.Context, skuIDs []int64) error {
	keys := make([]string, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		if skuID > 0 {
			keys = append(keys, c.skuKey(skuID))
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *ProductCache) productKey(productID int64) string {
	return fmt.Sprintf("%sproduct:%d", c.keyPrefix, productID)
}

func (c *ProductCache) skuKey(skuID int64) string {
	return fmt.Sprintf("%ssku:%d", c.keyPrefix, skuID)
}

func (c *ProductCache) productListKey(cacheKey string) string {
	return fmt.Sprintf("%sproduct_list:%s", c.keyPrefix, cacheKey)
}

func maxPositive(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
