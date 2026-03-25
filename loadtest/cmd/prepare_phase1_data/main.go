package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxAttempts       = 5
	rateLimitCode int32 = 1000005
)

type envelope struct {
	Code    int32           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	TraceID string          `json:"trace_id"`
}

type loginData struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type registerData struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type createProductData struct {
	ProductID int64               `json:"product_id"`
	SKUs      []createdProductSKU `json:"skus"`
}

type createdProductSKU struct {
	ID      int64  `json:"id"`
	SKUCode string `json:"sku_code"`
}

type credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type productSeed struct {
	Name      string `json:"name"`
	ProductID string `json:"product_id"`
	SKUID     string `json:"sku_id"`
	SKUCode   string `json:"sku_code"`
	Stock     int64  `json:"stock"`
}

type manifest struct {
	GeneratedAt    string        `json:"generated_at"`
	BaseURL        string        `json:"base_url"`
	Phase          string        `json:"phase"`
	Admin          credential    `json:"admin"`
	BuyerPassword  string        `json:"buyer_password"`
	Buyers         []credential  `json:"buyers"`
	HotProduct     productSeed   `json:"hot_product"`
	NormalProducts []productSeed `json:"normal_products"`
}

func main() {
	var (
		baseURL            = flag.String("base-url", "http://127.0.0.1:8080", "gateway base url")
		adminUsername      = flag.String("admin-username", "loadtest_superadmin", "superadmin username used for product seeding")
		adminPassword      = flag.String("admin-password", "Loadtest123456", "superadmin password used for product seeding")
		userPrefix         = flag.String("user-prefix", "loadtest_user", "prefix for generated buyer users")
		userPassword       = flag.String("user-password", "Loadtest123456", "password for generated buyer users")
		userCount          = flag.Int("user-count", 20, "number of buyer users to prepare")
		normalProductCount = flag.Int("normal-product-count", 5, "number of normal products to seed")
		normalStock        = flag.Int64("normal-stock", 200, "initial stock for each normal product")
		hotStock           = flag.Int64("hot-stock", 5000, "initial stock for the hot product")
		output             = flag.String("output", "", "optional manifest output path")
		timeout            = flag.Duration("timeout", 5*time.Second, "per-request timeout")
	)
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	runID := time.Now().Format("20060102-150405")

	adminCred := credential{Username: *adminUsername, Password: *adminPassword}
	adminLogin, err := ensureUserAndLogin(client, *baseURL, adminCred)
	if err != nil {
		fatalf("prepare admin user failed: %v", err)
	}

	buyers := make([]credential, 0, *userCount)
	for i := 1; i <= *userCount; i++ {
		cred := credential{
			Username: fmt.Sprintf("%s_%02d", *userPrefix, i),
			Password: *userPassword,
		}
		if _, err := ensureUserAndLogin(client, *baseURL, cred); err != nil {
			fatalf("prepare buyer user %q failed: %v", cred.Username, err)
		}
		buyers = append(buyers, cred)
	}

	hotProduct, err := createSeedProduct(client, *baseURL, adminLogin.AccessToken, seedProductRequest{
		Title:       "Loadtest Hot Product",
		SubTitle:    "Phase 1 hotspot",
		CategoryID:  1001,
		Brand:       "MeshCart",
		Description: "Hot product for phase 1 load test",
		Status:      2,
		SKUCode:     fmt.Sprintf("LOADTEST-HOT-%s", runID),
		SKUTitle:    "Loadtest Hot SKU",
		SalePrice:   199900,
		MarketPrice: 259900,
		InitialStock: *hotStock,
	})
	if err != nil {
		fatalf("seed hot product failed: %v", err)
	}

	normalProducts := make([]productSeed, 0, *normalProductCount)
	for i := 1; i <= *normalProductCount; i++ {
		req := seedProductRequest{
			Title:        fmt.Sprintf("Loadtest Product %02d", i),
			SubTitle:     "Phase 1 baseline",
			CategoryID:   1001,
			Brand:        "MeshCart",
			Description:  fmt.Sprintf("Normal product %02d for phase 1 load test", i),
			Status:       2,
			SKUCode:      fmt.Sprintf("LOADTEST-NORMAL-%02d-%s", i, runID),
			SKUTitle:     fmt.Sprintf("Loadtest SKU %02d", i),
			SalePrice:    99900 + int64(i*1000),
			MarketPrice:  129900 + int64(i*1000),
			InitialStock: *normalStock,
		}
		seed, err := createSeedProduct(client, *baseURL, adminLogin.AccessToken, req)
		if err != nil {
			fatalf("seed normal product %d failed: %v", i, err)
		}
		normalProducts = append(normalProducts, seed)
	}

	out := manifest{
		GeneratedAt:    time.Now().Format(time.RFC3339),
		BaseURL:        strings.TrimRight(*baseURL, "/"),
		Phase:          "phase1",
		Admin:          adminCred,
		BuyerPassword:  *userPassword,
		Buyers:         buyers,
		HotProduct:     hotProduct,
		NormalProducts: normalProducts,
	}

	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fatalf("marshal manifest failed: %v", err)
	}

	if *output != "" {
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			fatalf("create output dir failed: %v", err)
		}
		if err := os.WriteFile(*output, buf, 0o644); err != nil {
			fatalf("write manifest failed: %v", err)
		}
		fmt.Fprintf(os.Stderr, "manifest written to %s\n", *output)
	}

	fmt.Println(string(buf))
}

type seedProductRequest struct {
	Title        string
	SubTitle     string
	CategoryID   int64
	Brand        string
	Description  string
	Status       int32
	SKUCode      string
	SKUTitle     string
	SalePrice    int64
	MarketPrice  int64
	InitialStock int64
}

func ensureUserAndLogin(client *http.Client, baseURL string, cred credential) (*loginData, error) {
	registerReq := map[string]string{
		"username": cred.Username,
		"password": cred.Password,
	}
	var regResp registerData
	err := postJSONWithRetry(client, baseURL+"/api/v1/user/register", "", registerReq, &regResp)
	if err != nil && !isAlreadyExistsError(err) {
		return nil, err
	}

	loginReq := map[string]string{
		"username": cred.Username,
		"password": cred.Password,
	}
	var loginResp loginData
	if err := postJSONWithRetry(client, baseURL+"/api/v1/user/login", "", loginReq, &loginResp); err != nil {
		return nil, err
	}
	return &loginResp, nil
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "已存在") || strings.Contains(strings.ToLower(msg), "exist")
}

func createSeedProduct(client *http.Client, baseURL, accessToken string, req seedProductRequest) (productSeed, error) {
	body := map[string]any{
		"title":       req.Title,
		"sub_title":   req.SubTitle,
		"category_id": req.CategoryID,
		"brand":       req.Brand,
		"description": req.Description,
		"status":      req.Status,
		"skus": []map[string]any{
			{
				"sku_code":      req.SKUCode,
				"title":         req.SKUTitle,
				"sale_price":    req.SalePrice,
				"market_price":  req.MarketPrice,
				"status":        1,
				"cover_url":     "https://example.test/loadtest.png",
				"initial_stock": req.InitialStock,
				"attrs": []map[string]any{
					{"attr_name": "颜色", "attr_value": "黑色", "sort": 1},
					{"attr_name": "版本", "attr_value": "标准版", "sort": 2},
				},
			},
		},
	}

	var data createProductData
	if err := postJSONWithRetry(client, baseURL+"/api/v1/admin/products", accessToken, body, &data); err != nil {
		return productSeed{}, err
	}
	if len(data.SKUs) == 0 {
		return productSeed{}, errors.New("create product returned empty sku list")
	}
	return productSeed{
		Name:      req.Title,
		ProductID: fmt.Sprintf("%d", data.ProductID),
		SKUID:     fmt.Sprintf("%d", data.SKUs[0].ID),
		SKUCode:   data.SKUs[0].SKUCode,
		Stock:     req.InitialStock,
	}, nil
}

func postJSON(client *http.Client, url, accessToken string, reqBody any, respBody any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		if strings.HasPrefix(accessToken, "Bearer ") {
			req.Header.Set("Authorization", accessToken)
		} else {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("decode response failed: %w, body=%s", err, string(body))
	}
	if env.Code != 0 {
		return &bizError{Code: env.Code, Message: env.Message, TraceID: env.TraceID}
	}
	if respBody == nil {
		return nil
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, respBody); err != nil {
		return fmt.Errorf("decode business data failed: %w", err)
	}
	return nil
}

type bizError struct {
	Code    int32
	Message string
	TraceID string
}

func (e *bizError) Error() string {
	return fmt.Sprintf("business error code=%d message=%s trace_id=%s", e.Code, e.Message, e.TraceID)
}

func postJSONWithRetry(client *http.Client, url, accessToken string, reqBody any, respBody any) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := postJSON(client, url, accessToken, reqBody, respBody)
		if err == nil {
			return nil
		}
		lastErr = err
		var bizErr *bizError
		if !errors.As(err, &bizErr) || bizErr.Code != rateLimitCode || attempt == maxAttempts {
			return err
		}
		time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
	}
	return lastErr
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
